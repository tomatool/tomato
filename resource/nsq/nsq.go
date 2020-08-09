package nsq

import (
	"fmt"
	"sync"
	"time"

	"github.com/nsqio/go-nsq"
	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/stub"
)

const (
	DefaultWaitDuration = time.Millisecond * 50

	// suffix for nsq topic/channel to make them ephemeral
	Ephemeral = "#ephemeral"
)

// dummyDelegate is used to check nsqd connnection
type dummyDelegate int

func (d dummyDelegate) OnResponse(c *nsq.Conn, data []byte)           {}
func (d dummyDelegate) OnError(c *nsq.Conn, data []byte)              {}
func (d dummyDelegate) OnMessage(c *nsq.Conn, m *nsq.Message)         {}
func (d dummyDelegate) OnMessageFinished(c *nsq.Conn, m *nsq.Message) {}
func (d dummyDelegate) OnMessageRequeued(c *nsq.Conn, m *nsq.Message) {}
func (d dummyDelegate) OnBackoff(c *nsq.Conn)                         {}
func (d dummyDelegate) OnContinue(c *nsq.Conn)                        {}
func (d dummyDelegate) OnResume(c *nsq.Conn)                          {}
func (d dummyDelegate) OnIOError(c *nsq.Conn, err error)              {}
func (d dummyDelegate) OnHeartbeat(c *nsq.Conn)                       {}
func (d dummyDelegate) OnClose(c *nsq.Conn)                           {}

// silentLogger is used to suppress nsq log
type silentLogger int

func (sl silentLogger) Output(calldepth int, s string) error { return nil }

var theSilentLogger = silentLogger(1)

type NSQ struct {
	sync.Mutex
	nsqd           string
	producer       *nsq.Producer
	topicReadiness map[string]*nsq.Consumer
	data           map[string][][]byte
	waitDuration   time.Duration
	stubs          *stub.Stubs
}

func New(cfg *config.Resource) (*NSQ, error) {
	nsqdAddress, ok := cfg.Options["nsqd"]
	if !ok {
		return nil, errors.New("queue/nsq: nsqd address is required")
	}

	waitDuration, err := time.ParseDuration(cfg.Options["wait_duration"])
	if err != nil {
		waitDuration = DefaultWaitDuration
	}

	path, ok := cfg.Options["stubs_path"]
	stubs := &stub.Stubs{}
	if ok {
		var err error
		stubs, err = stub.RetrieveFiles(path)
		if err != nil {
			return nil, err
		}
	}

	return &NSQ{
		nsqd:           nsqdAddress,
		waitDuration:   waitDuration,
		topicReadiness: make(map[string]*nsq.Consumer),
		data:           make(map[string][][]byte),
		stubs:          stubs,
	}, nil
}

// Open satisfies resource interface
func (nm *NSQ) Open() error {
	return nil
}

// to check whether nsq is ready to be used, both for publishing or consuming.
// it is intended not to be cached
func (nm *NSQ) Ready() error {
	dummyTarget := "ready_flag"

	err := nm.Publish(dummyTarget, []byte("test"))
	if err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to publish test")
	}

	cNsq, err := nm.connectConsumer(dummyTarget)
	if err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to attach testing consumer")
	}

	cNsq.Stop()

	return nil
}

func (nm *NSQ) Reset() error {
	nm.data = make(map[string][][]byte)
	return nil
}

// to terminate nsq connections
func (nm *NSQ) Close() error {
	nm.Lock()
	for _, v := range nm.topicReadiness {
		if v != nil {
			v.Stop()
		}
	}
	nm.Unlock()

	return nil
}

// to connect consumer function to target
func (nm *NSQ) connectConsumer(target string) (*nsq.Consumer, error) {
	cNsq, err := nsq.NewConsumer(target+Ephemeral, "tomato"+Ephemeral, nsq.NewConfig())
	if err != nil {
		return nil, err
	}

	// suppress nsq log, log level won't matter
	cNsq.SetLogger(theSilentLogger, nsq.LogLevelDebug)

	cNsq.AddHandler(nsq.HandlerFunc(func(n *nsq.Message) error {
		nm.Lock()
		_, exist := nm.data[target]
		if !exist {
			nm.data[target] = make([][]byte, 0)
		}
		nm.data[target] = append(nm.data[target], n.Body)
		nm.Unlock()

		// always happy case, always finish the message
		n.Finish()
		return nil
	}))

	if err := cNsq.ConnectToNSQD(nm.nsqd); err != nil {
		return nil, err
	}

	// this needed to clear up messages that got published before queue exist
	time.Sleep(time.Millisecond * 200)
	nm.data[target] = make([][]byte, 0)

	return cNsq, nil
}

// to listen to target, all messages retrieved should be stored temporarily
func (nm *NSQ) Listen(target string) error {

	nm.Lock()
	cNsqCached, exist := nm.topicReadiness[target]
	nm.Unlock()

	if exist && cNsqCached != nil {
		return nil
	}

	// if there hasn't any, try to connect consumer
	cNsq, err := nm.connectConsumer(target)
	if err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to attach consumer")
	}
	nm.data[target] = make([][]byte, 0)
	nm.Lock()
	nm.topicReadiness[target] = cNsq
	nm.Unlock()

	return nil
}

// to initiate producer if hasn't
func (nm *NSQ) introduceProducer() error {
	if nm.producer != nil {
		return nil
	}

	nsqProducer, err := nsq.NewProducer(nm.nsqd, nsq.NewConfig())
	if err != nil {
		return err
	}

	nsqProducer.SetLogger(theSilentLogger, nsq.LogLevelDebug)

	nm.producer = nsqProducer

	return nil
}

// PublishFromFile attempts to publish a message read from a file to the passed target
func (nm *NSQ) PublishFromFile(target string, fileName string) error {
	payload, err := nm.stubs.Get(fileName)
	if err != nil {
		return err
	}
	return nm.Publish(target, payload)
}

// to publish payload to designated target
func (nm *NSQ) Publish(target string, payload []byte) error {
	if err := nm.introduceProducer(); err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to initiate producer")
	}

	if err := nm.producer.Publish(target+Ephemeral, payload); err != nil {
		return err
	}
	time.Sleep(nm.waitDuration)
	return nil
}

// to get the message from target that are stored by nsqManager.Listen()
func (nm *NSQ) Fetch(target string) ([][]byte, error) {
	nm.Lock()
	defer nm.Unlock()

	msgs, ok := nm.data[target]
	if !ok {
		return nil, fmt.Errorf("queue not exist, please listen to it %s, %+v", target, nm.data)
	}

	return msgs, nil
}
