package queue

import (
	"errors"

	"github.com/alileza/tomato/resource/queue/nsq"
	"github.com/alileza/tomato/resource/queue/rabbitmq"
)

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

func Open(params map[string]string) (Client, error) {
	driver, ok := params["driver"]
	if !ok {
		return nil, errors.New("queue: driver is required")
	}
	switch driver {
	case "rabbitmq":
		return rabbitmq.Open(params)
	case "nsq":
		return nsq.Open(params)
	}
	return nil, errors.New("queue: invalid driver > " + driver)
}
