package handler

import "github.com/alileza/tomato/resource"

var (
	resourceManager = &resourceManagerMock{}
	h               = Handler{resourceManager}
)

type resourceManagerMock struct{}

func (mgr *resourceManagerMock) Get(name string) (resource.Resource, error) {
	switch name {
	case "sql-resource":
		return resourceSQL, nil
	case "httpcli-resource":
		return resourceHTTPClient, nil
	case "httpsrv-resource":
		return resourceHTTPServer, nil
	case "queue-resource":
		return resourceQueue, nil
	}

	return nil, nil
}
