package queue

import "github.com/alileza/tomato/resource/queue/rabbitmq"

const Name = "queue"

type Client interface {
	Listen(target string) error
	Count(target string) (int, error)
	Publish(target string, payload []byte) error
	Consume(target string) []byte
	Ready() error
	Close() error
}

func Cast(i interface{}) Client {
	return i.(Client)
}

func New(params map[string]string) Client {
	driver, ok := params["driver"]
	if !ok {
		panic("queue: driver is required")
	}
	switch driver {
	case "rabbitmq":
		return rabbitmq.New(params)
	}
	panic("queue: invalid driver > " + driver)
}
