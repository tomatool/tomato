package nsq

import (
	"sync"
	"time"

	"github.com/bitly/go-nsq"
	"github.com/pkg/errors"
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

type nsqManager struct {
	sync.Mutex
	nsqd           string
	producer       *nsq.Producer
	topicReadiness map[string]*nsq.Consumer
	data           map[string][][]byte
	waitDuration   time.Duration
}

func Open(params map[string]string) (*nsqManager, error) {
	nsqdAddress, ok := params["nsqd"]
	if !ok {
		return nil, errors.New("queue/nsq: nsqd address is required")
	}

	waitDuration, err := time.ParseDuration(params["wait_duration"])
	if err != nil {
		waitDuration = DefaultWaitDuration
	}

	// testing nsqd connection with dummyDelegate
	nsqdConn := nsq.NewConn(nsqdAddress, nsq.NewConfig(), dummyDelegate(1))
	if _, err := nsqdConn.Connect(); err != nil {
		return nil, errors.New("queue/nsq: failed to connect to nsqd")
	}
	nsqdConn.Flush()
	nsqdConn.Close()
	if !nsqdConn.IsClosing() {
		return nil, errors.New("queue/nsq: there's problem with nsqd conn")
	}

	return &nsqManager{
		nsqd:           nsqdAddress,
		waitDuration:   waitDuration,
		topicReadiness: make(map[string]*nsq.Consumer),
		data:           make(map[string][][]byte),
	}, nil
}

// to connect consumer function to target
func (nm *nsqManager) connectConsumer(target string) (*nsq.Consumer, error) {
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

	return cNsq, nil
}

// to listen to target, all messages retrieved should be stored temporarily
func (nm *nsqManager) Listen(target string) error {
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

	nm.Lock()
	nm.topicReadiness[target] = cNsq
	nm.Unlock()

	return nil
}

// to count number of message already delivered through target
func (nm *nsqManager) Count(target string) (int, error) {
	nm.Lock()
	msgs, exist := nm.data[target]
	nm.Unlock()

	if !exist {
		return 0, errors.New("queue/nsq: no consumer registered")
	}

	return len(msgs), nil
}

// to initiate producer if hasn't
func (nm *nsqManager) introduceProducer() error {
	if nm.producer != nil {
		return nil
	}

	nsqProducer, err := nsq.NewProducer(nm.nsqd, nsq.NewConfig())
	if err != nil {
		return err
	}

	nm.producer = nsqProducer

	return nil
}

// to publish payload to designated target
func (nm *nsqManager) Publish(target string, payload []byte) error {
	if err := nm.introduceProducer(); err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to initiate producer")
	}

	return nm.producer.Publish(target+Ephemeral, payload)
}

// to get the message from target that are stored by nsqManager.Listen()
func (nm *nsqManager) Consume(target string) []byte {
	nm.Lock()
	msgs, exist := nm.data[target]
	nm.Unlock()

	if !exist || msgs == nil {
		return nil
	}

	if len(msgs) < 1 {
		return nil
	}

	msg := msgs[0]

	// removing already read message
	nm.Lock()
	nm.data[target] = msgs[1:]
	nm.Unlock()

	return msg
}

// to check whether nsq is ready to be used, both for publishing or consuming.
// it is intended not to be cached
func (nm *nsqManager) Ready() error {
	dummyTarget := "ready_flag"

	err := nm.Publish(dummyTarget, []byte(""))
	if err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to publish test")
	}

	cNsq, err := nm.connectConsumer(dummyTarget)
	if err != nil {
		return errors.Wrapf(err, "queue/nsq: failed to attach testing consumer")
	}

	res := nm.Consume(dummyTarget)
	if res == nil {
		return errors.New("queue/nsq: not ready yet")
	}

	cNsq.Stop()

	return nil
}

// to terminate nsq connections
func (nm *nsqManager) Close() error {
	nm.Lock()
	for _, v := range nm.topicReadiness {
		if v != nil {
			v.Stop()
		}
	}
	nm.Unlock()

	return nil
}
