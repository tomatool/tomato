package errors

type Step struct {
	Description string
	Details     map[string]string
}

func (sf *Step) Error() string {
	return sf.Description
}

func NewStep(desc string, details map[string]string) *Step {
	return &Step{desc, details}
}
