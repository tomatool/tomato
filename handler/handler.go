/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package handler

import (
	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource"
)

type Handler struct {
	resource *resource.Manager
}

func New(r *resource.Manager) func(s *godog.Suite) {
	h := &Handler{r}
	return func(s *godog.Suite) {
		s.BeforeFeature(func(_ *gherkin.Feature) {
			h.resource.Reset()
		})
		s.AfterScenario(func(_ interface{}, _ error) {
			h.resource.Reset()
		})
		s.Step(`^"([^"]*)" send request to "([^"]*)"$`, h.sendRequest)
		s.Step(`^"([^"]*)" send request to "([^"]*)" with body$`, h.sendRequestWithBody)
		s.Step(`^"([^"]*)" send request to "([^"]*)" with payload$`, h.sendRequestWithBody)
		s.Step(`^"([^"]*)" response code should be (\d+)$`, h.checkResponseCode)
		s.Step(`^"([^"]*)" response body should be$`, h.checkResponseBody)
		s.Step(`^set "([^"]*)" response code to (\d+) and response body$`, h.setResponse)
		s.Step(`^set "([^"]*)" with path "([^"]*)" response code to (\d+) and response body$`, h.setResponse)
		s.Step(`^set "([^"]*)" table "([^"]*)" list of content$`, h.tableInsert)
		s.Step(`^"([^"]*)" table "([^"]*)" should look like$`, h.tableCompare)
		s.Step(`^publish message to "([^"]*)" target "([^"]*)" with payload$`, h.publishMessage)
		s.Step(`^listen message from "([^"]*)" target "([^"]*)"$`, h.listenMessage)
		s.Step(`^message from "([^"]*)" target "([^"]*)" count should be (\d+)$`, h.countMessage)
		s.Step(`^message from "([^"]*)" target "([^"]*)" should look like$`, h.compareMessage)
		s.Step(`^"([^"]*)" execute "([^"]*)"$`, h.execCommand)
		s.Step(`^"([^"]*)" stdout should contains "([^"]*)"$`, h.checkStdoutContains)
		s.Step(`^"([^"]*)" stdout should not contains "([^"]*)"$`, h.checkStdoutNotContains)
		s.Step(`^"([^"]*)" stderr should contains "([^"]*)"$`, h.checkStderrContains)
		s.Step(`^"([^"]*)" stderr should not contains "([^"]*)"$`, h.checkStderrNotContains)

    }
}