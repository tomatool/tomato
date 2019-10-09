/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package sql

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^set "([^"]*)" table "([^"]*)" list of content$`, h.tableInsert)
	s.Step(`^"([^"]*)" table "([^"]*)" should look like$`, h.tableCompare)
}
