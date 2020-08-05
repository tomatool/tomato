package dictionary_test

import (
	"testing"

	"github.com/tomatool/tomato/dictionary"
)

func TestDictionarySchema(t *testing.T) {
	d, err := dictionary.Retrieve("../dictionary.yml")
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Handlers) == 0 {
		t.Fatalf("Expecting dictionary handlers > 0, got %d", len(d.Handlers))
	}

	handlerNames := make(map[string]struct{})
	handlerDescriptions := make(map[string]struct{})
	for _, handler := range d.Handlers {
		if _, ok := handlerNames[handler.Name]; ok {
			t.Errorf("Duplicate handler name of `%s` in dictionary", handler.Name)
		} else {
			handlerNames[handler.Name] = struct{}{}
		}

		if _, ok := handlerDescriptions[handler.Description]; ok {
			t.Errorf("Duplicate handler name of `%s` in dictionary", handler.Name)
		} else {
			handlerDescriptions[handler.Name] = struct{}{}
		}

		if len(handler.Resources) == 0 {
			t.Errorf("Unexpected empty resources on handler %s", handler.Name)
		}

		actionNames := make(map[string]struct{})
		actionDescriptions := make(map[string]struct{})
		for _, action := range handler.Actions {
			if _, ok := actionNames[action.Name]; ok {
				t.Errorf("Duplicate handler action name of `%s` in dictionary handler `%s`", action.Name, handler.Name)
			} else {
				actionNames[action.Name] = struct{}{}
			}

			if _, ok := actionDescriptions[action.Description]; ok {
				t.Errorf("Duplicate handler description name of `%s` in dictionary handler `%s`", action.Name, handler.Name)
			} else {
				actionDescriptions[action.Description] = struct{}{}
			}

			if len(action.Expressions) == 0 {
				t.Errorf("Unexpected empty expressions for handler `%s` action `%s`", handler.Name, action.Name)
			}

			if len(action.Handle) == 0 {
				t.Errorf("Unexpected empty handle for handler `%s` action `%s`", handler.Name, action.Name)
			}

			// check if handle is valid func name

			// check if action.Params match with expression

		}
	}
}
