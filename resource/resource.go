package resource

type Resource interface {
	Ready() error
	Reset() error
}
