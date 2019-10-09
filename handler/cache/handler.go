/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package cache

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^cache "([^"]*)" stores "([^"]*)" with value "([^"]*)"$`, h.valueSet)
	s.Step(`^cache "([^"]*)" stored key "([^"]*)" should look like "([^"]*)"$`, h.valueCompare)
	s.Step(`^cache "([^"]*)" has key "([^"]*)"$`, h.valueExists)
	s.Step(`^cache "([^"]*)" hasn't key "([^"]*)"$`, h.valueNotExists)
}
