package resource

type Handler interface {
	Open() error
	Ready() error
	Reset() error
	Close() error
}
