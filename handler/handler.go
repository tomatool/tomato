/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the cmd/gen/main.go tool */
package handler

import (
	"github.com/DATA-DOG/godog"
	"github.com/alileza/tomato/resource"
)

type Handler struct {
	resource *resource.Manager
}

func New(r *resource.Manager) func(s *godog.Suite) {
	h := &Handler{r}
	return func(s *godog.Suite) {
		s.BeforeScenario(func(_ interface{}) {
			h.resource.Reset()
		})

		s.Step(`^"([^"]*)" send request to "([^"]*)"$`, h.sendRequest)
		s.Step(`^"([^"]*)" send request to "([^"]*)" with body$`, h.sendRequestWithBody)
		s.Step(`^"([^"]*)" response code should be (\d+)$`, h.checkResponseCode)
		s.Step(`^"([^"]*)" response body should be$`, h.checkResponseBody)
		s.Step(`^set "([^"]*)" with path "([^"]*)" response code to (\d+) and response body$`, h.setResponse)
		s.Step(`^set "([^"]*)" table "([^"]*)" list of content$`, h.tableInsert)
		s.Step(`^"([^"]*)" table "([^"]*)" should look like$`, h.tableCompare)
		s.Step(`^publish message to "([^"]*)" target "([^"]*)" with payload$`, h.publishMessage)
		s.Step(`^listen message from "([^"]*)" target "([^"]*)"$`, h.listenMessage)
		s.Step(`^message from "([^"]*)" target "([^"]*)" count should be (\d+)$`, h.countMessage)
		s.Step(`^message from "([^"]*)" target "([^"]*)" should look like$`, h.messageCompare)
	}
}
