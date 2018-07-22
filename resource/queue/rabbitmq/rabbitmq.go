package rabbitmq

const Name = "queue/rabbitmq"

type Client struct{}

func T(i interface{}) *Client {
	return i.(*Client)
}

func New() {

}
