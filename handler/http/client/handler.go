/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package client

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^"([^"]*)" send request to "([^"]*)"$`, h.sendRequest)
	s.Step(`^"([^"]*)" send request to "([^"]*)" with body$`, h.sendRequestWithBody)
	s.Step(`^"([^"]*)" send request to "([^"]*)" with payload$`, h.sendRequestWithBody)
	s.Step(`^"([^"]*)" send request to "([^"]*)" with body from file "([^"]*)"$`, h.sendRequestWithBodyFromFile)
	s.Step(`^"([^"]*)" send request to "([^"]*)" with payload from file "([^"]*)"$`, h.sendRequestWithBodyFromFile)
	s.Step(`^"([^"]*)" set request header key "([^"]*)" with value "([^"]*)"$`, h.setRequestHeader)
	s.Step(`^"([^"]*)" response code should be (\d+)$`, h.checkResponseCode)
	s.Step(`^"([^"]*)" response header "([^"]*)" should be "([^"]*)"$`, h.checkResponseHeader)
	s.Step(`^"([^"]*)" response body should contain$`, h.checkResponseBodyContains)
	s.Step(`^"([^"]*)" response body should equal$`, h.checkResponseBodyEquals)
}
