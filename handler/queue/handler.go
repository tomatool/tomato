/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package queue

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^publish message to "([^"]*)" target "([^"]*)" with payload$`, h.publishMessage)
	s.Step(`^publish message to "([^"]*)" target "([^"]*)" with payload from file "([^"]*)"$`, h.publishMessageFromFile)
	s.Step(`^listen message from "([^"]*)" target "([^"]*)"$`, h.listenMessage)
	s.Step(`^message from "([^"]*)" target "([^"]*)" count should be (\d+)$`, h.countMessage)
	s.Step(`^message from "([^"]*)" target "([^"]*)" should contain$`, h.compareMessageContains)
	s.Step(`^message from "([^"]*)" target "([^"]*)" should equal$`, h.compareMessageEquals)
}