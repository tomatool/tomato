package handler

import "fmt"

type ErrMismatch struct {
	field       string
	expectation interface{}
	result      interface{}
	metadata    string
}

func (e *ErrMismatch) Error() string {
	msg := fmt.Sprintf("\n[MISMATCH] %s\nexpecting\t:\t%+v\ngot\t\t:\t%+v", e.field, e.expectation, e.result)
	if e.metadata != "" {
		msg += fmt.Sprintf("\nmetadata\t:\t%s", e.metadata)
	}
	return msg
}
