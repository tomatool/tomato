package rabbitmq

import (
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

const (
	DefaultWaitDuration = time.Millisecond * 50
)

type rabbitMQ struct {
	mtx             sync.Mutex
	conn            *amqp.Connection
	consumedMessage map[string][][]byte
	waitDuration    time.Duration
}

func Open(params map[string]string) (*rabbitMQ, error) {
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

	return &rabbitMQ{conn: conn, waitDuration: waitDuration}, nil
}

func (c *rabbitMQ) target(target string) (string, string) {
	result := strings.Split(target, ":")
	if len(result) < 2 {
		return result[0], ""
	}
	return result[0], result[1]
}

func (c *rabbitMQ) Ready() error {
	if err := c.Publish("test:test", []byte("")); err != nil {
		return errors.Wrapf(err, "queue: rabbitmq is not ready")
	}
	return nil
}

func (c *rabbitMQ) Close() error {
	return c.conn.Close()
}

func (c *rabbitMQ) Listen(target string) error {
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
	if _, ok := c.consumedMessage[target]; ok {
		c.consumedMessage[target] = make([][]byte, 0)
	}

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

func (c *rabbitMQ) clear(target string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.consumedMessage[target] = [][]byte{}
}

func (c *rabbitMQ) consume(target string, payload []byte) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.consumedMessage == nil {
		c.consumedMessage = make(map[string][][]byte)
	}

	c.consumedMessage[target] = append(c.consumedMessage[target], payload)
}

func (c *rabbitMQ) Publish(target string, payload []byte) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

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

func (c *rabbitMQ) Count(target string) (int, error) {
	time.Sleep(c.waitDuration)
	return len(c.consumedMessage[target]), nil
}

func (c *rabbitMQ) Consume(target string) []byte {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	msgs, ok := c.consumedMessage[target]
	if !ok {
		return nil
	}

	msg := msgs[0]

	// removing read messaged
	c.consumedMessage[target] = msgs[1:]

	return msg
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
