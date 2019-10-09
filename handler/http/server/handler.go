/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package server

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^set "([^"]*)" response code to (\d+) and response body$`, h.setResponse)
	s.Step(`^set "([^"]*)" response code to (\d+) and response body from file "([^"]*)"$`, h.setResponseFromFile)
	s.Step(`^set "([^"]*)" with path "([^"]*)" response code to (\d+) and response body$`, h.setResponse)
	s.Step(`^set "([^"]*)" with method "([^"]*)" and path "([^"]*)" response code to (\d+) and response body$`, h.setResponseWithMethod)
	s.Step(`^set "([^"]*)" with method "([^"]*)" and path "([^"]*)" response code to (\d+)$`, h.setResponseWithMethodAndNoBody)
	s.Step(`^set "([^"]*)" with method "([^"]*)" and path "([^"]*)" response code to (\d+) and response body from file "([^"]*)"$`, h.setResponseWithMethodAndBodyFromFile)
	s.Step(`^"([^"]*)" with path "([^"]*)" request count should be (\d+)$`, h.verifyRequestsCount)
}
