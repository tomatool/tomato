/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package kv

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^"([^"]*)" set "([^"]*)" to "([^"]*)"$`, h.set)
	s.Step(`^"([^"]*)" key "([^"]*)" should equal "([^"]*)"$`, h.compare)
}