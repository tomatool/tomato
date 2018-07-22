package rabbitmq

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

type RabbitMQ struct {
	mtx             sync.Mutex
	conn            *amqp.Connection
	consumedMessage map[string][][]byte
}

func New(params map[string]string) *RabbitMQ {
	datasource, ok := params["datasource"]
	if !ok {
		panic("queue/rabbitmq: datasource is required")
	}

	conn, err := amqp.Dial(datasource)
	if err != nil {
		panic("queue/rabbitmq: failed to connect > " + err.Error())
	}
	return &RabbitMQ{conn: conn}
}

func (c *RabbitMQ) target(target string) (string, string) {
	result := strings.Split(target, ":")
	if len(result) < 2 {
		panic("queue/rabbitmq: invalid target format")
	}
	return result[0], result[1]
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

	if c.consumedMessage == nil {
		c.consumedMessage = make(map[string][][]byte)
	}

	c.consumedMessage[target] = append(c.consumedMessage[target], payload)
}

func (c *RabbitMQ) Publish(target string, payload []byte) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	exchange, key := c.target(target)
	if err := exchangeDeclare(ch, exchange); err != nil {
		return err
	}

	return ch.Publish(
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
}

func (c *RabbitMQ) Count(target string, count int) error {
	time.Sleep(time.Millisecond * 50)

	o := len(c.consumedMessage[target])
	if o == count {
		return nil
	}

	return fmt.Errorf("queue/rabbitmq: mismatch count for target `%s`, expecting=%d got=%d", target, count, o)
}

func (c *RabbitMQ) Message(target string) []byte {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	msgs, ok := c.consumedMessage[target]
	if !ok {
		return nil
	}

	msg := msgs[0]
	msgs = msgs[1:]

	return msg
}

func exchangeDeclare(ch *amqp.Channel, name string) error {
	return ch.ExchangeDeclare(
		name,     // name of the exchange
		"direct", // type
		true,     // durable
		false,    // delete when complete
		false,    // internal
		false,    // noWait
		nil,      // arguments
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
