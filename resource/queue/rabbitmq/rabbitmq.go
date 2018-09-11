package rabbitmq

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alileza/tomato/config"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

const (
	DefaultWaitDuration = time.Millisecond * 50
)

type RabbitMQ struct {
	mtx             sync.Mutex
	conn            *amqp.Connection
	consumedMessage map[string][][]byte
	waitDuration    time.Duration
}

func New(cfg *config.Resource) (*RabbitMQ, error) {
	params := cfg.Params
	datasource, ok := params["datasource"]
	if !ok {
		return nil, errors.New("queue/rabbitmq: datasource is required")
	}

	waitDuration, err := time.ParseDuration(params["wait_duration"])
	if err != nil {
		waitDuration = DefaultWaitDuration
	}

	conn, err := amqp.Dial(datasource)
	if err != nil {
		return nil, errors.New("queue/rabbitmq: failed to connect > " + err.Error())
	}

	return &RabbitMQ{
		conn:            conn,
		waitDuration:    waitDuration,
		consumedMessage: make(map[string][][]byte),
	}, nil
}

func (c *RabbitMQ) target(target string) (string, string) {
	result := strings.Split(target, ":")
	if len(result) < 2 {
		return result[0], ""
	}
	return result[0], result[1]
}

func (c *RabbitMQ) Ready() error {
	if err := c.Publish("test:test", []byte("")); err != nil {
		return errors.Wrapf(err, "queue: rabbitmq is not ready")
	}
	return nil
}

func (c *RabbitMQ) Reset() error {
	c.consumedMessage = make(map[string][][]byte)
	return nil
}

func (c *RabbitMQ) Listen(target string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	exchange, key := c.target(target)

	if err := exchangeDeclare(ch, exchange); err != nil {
		return err
	}

	q, err := queueDeclare(ch, exchange+"."+key+".tmp.queue")
	if err != nil {
		return err
	}

	err = queueBind(ch, q.Name, exchange, key)
	if err != nil {
		return err
	}

	if _, err := ch.QueuePurge(q.Name, false); err != nil {
		return err
	}
	c.consumedMessage[target] = make([][]byte, 0)

	msgs, err := consume(ch, q.Name)
	if err != nil {
		return err
	}

	go func(msgs <-chan amqp.Delivery, target string) {
		for d := range msgs {
			c.consume(target, d.Body)
		}
	}(msgs, target)

	return nil
}

func (c *RabbitMQ) clear(target string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.consumedMessage[target] = [][]byte{}
}

func (c *RabbitMQ) consume(target string, payload []byte) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.consumedMessage[target] = append(c.consumedMessage[target], payload)
}

func (c *RabbitMQ) Publish(target string, payload []byte) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	exchange, key := c.target(target)
	if err := exchangeDeclare(ch, exchange); err != nil {
		return err
	}

	err = ch.Publish(
		exchange,
		key,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Transient,
			ContentType:  "application/json",
			Body:         payload,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return err
	}
	time.Sleep(c.waitDuration)
	return nil
}

func (c *RabbitMQ) Fetch(target string) ([][]byte, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	msgs, ok := c.consumedMessage[target]
	if !ok {
		return nil, fmt.Errorf("queue not exist, please listen to it %s, %+v", target, c.consumedMessage)
	}

	return msgs, nil
}

func exchangeDeclare(ch *amqp.Channel, name string) error {
	return ch.ExchangeDeclare(
		name,    // name of the exchange
		"topic", // type
		true,    // durable
		false,   // delete when complete
		false,   // internal
		false,   // noWait
		nil,     // arguments
	)
}

func queueDeclare(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,  // name, leave empty to generate a unique name
		true,  // durable
		false, // delete when usused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
}

func queueBind(ch *amqp.Channel, queue, exchange, key string) error {
	return ch.QueueBind(
		queue,    // name of the queue
		key,      // bindingKey
		exchange, // sourceExchange
		false,    // noWait
		nil,      // arguments
	)
}

func consume(ch *amqp.Channel, queue string) (<-chan amqp.Delivery, error) {
	return ch.Consume(
		queue,
		"",
		true,
		false,
		false,
		false,
		nil)
}
