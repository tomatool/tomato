/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package shell

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {
	s.Step(`^"([^"]*)" execute "([^"]*)"$`, h.execCommand)
	s.Step(`^"([^"]*)" stdout should contains "([^"]*)"$`, h.checkStdoutContains)
	s.Step(`^"([^"]*)" stdout should not contains "([^"]*)"$`, h.checkStdoutNotContains)
	s.Step(`^"([^"]*)" stderr should contains "([^"]*)"$`, h.checkStderrContains)
	s.Step(`^"([^"]*)" stderr should not contains "([^"]*)"$`, h.checkStderrNotContains)
	s.Step(`^"([^"]*)" exit code equal to (\d+)$`, h.checkExitCodeEqual)
	s.Step(`^"([^"]*)" exit code not equal to (\d+)$`, h.checkExitCodeNotEqual)
}
