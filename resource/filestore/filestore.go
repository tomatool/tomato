package filestore

import (
	"errors"

	"github.com/alileza/tomato/resource/filestore/s3"
)

const Name = "filestore"

type Client interface {
	List() (interface{}, error)
	Download(folder, file, outputFile string) error
	Upload(target string, payload []byte) (int, error)
	Delete(target string) error
	Ready() error
	Close() error
}

func Cast(i interface{}) Client {
	return i.(Client)
}

func Open(params map[string]string) (Client, error) {
	driver, ok := params["driver"]
	if !ok {
		return nil, errors.New("filestore: driver is required")
	}
	switch driver {
	case "s3":
		return s3.Open(params)
	}
	return nil, errors.New("queue: invalid driver > " + driver)
}
