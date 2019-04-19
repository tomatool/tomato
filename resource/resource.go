package resource

type Resource interface {
	Open() error
	Ready() error
	Reset() error
	Close() error
}
