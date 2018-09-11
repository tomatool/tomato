package compare

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func JSON(a []byte, b []byte, exact bool) error {
	mapA := make(map[string]interface{})
	if err := json.Unmarshal(a, &mapA); err != nil {
		return err
	}

	mapB := make(map[string]interface{})
	if err := json.Unmarshal(b, &mapB); err != nil {
		return err
	}

	if err := Map(mapA, mapB); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", string(a), string(b), err.Error())
	}

	if exact {
		if err := Map(mapB, mapA); err != nil {
			return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", string(a), string(b), err.Error())
		}
	}

	return nil
}

func Map(expectedResponse, gotResponse map[string]interface{}) error {
	for key := range expectedResponse {
		expectedVal, ok1 := expectedResponse[key]
		gotVal, ok2 := gotResponse[key]
		if ok1 != ok2 {
			return fmt.Errorf("mismatch field key='%s' expected='%v' got='%v'", key, ok1, ok2)
		}

		if err := Val(expectedVal, gotVal); err != nil {
			return fmt.Errorf("[%s] %s", key, err.Error())
		}
	}
	return nil
}

func Val(expectedVal, gotVal interface{}) error {
	expectedType := reflect.TypeOf(expectedVal)
	if expectedType != nil &&
		expectedType.Kind() == reflect.String &&
		expectedVal.(string) == "*" {
		return nil
	}

	gotType := reflect.TypeOf(gotVal)
	if expectedType != gotType {
		return fmt.Errorf("mismatch value type expected='%v' got='%v'", expectedType, gotType)
	}
	if expectedType == nil {
		return nil
	}

	if expectedType.Kind() == reflect.Slice {
		if err := Slice(
			expectedVal.([]interface{}),
			gotVal.([]interface{}),
		); err != nil {
			return err
		}
		return nil
	}

	if expectedType.Kind() == reflect.Map {
		if err := Map(
			expectedVal.(map[string]interface{}),
			gotVal.(map[string]interface{}),
		); err != nil {
			return err
		}
		return nil
	}

	if expectedVal != gotVal {
		return fmt.Errorf("mismatch value expected='%v' got='%v'", expectedVal, gotVal)
	}
	return nil
}

func Slice(expectedResponse, gotResponse []interface{}) error {
	if len(expectedResponse) != len(gotResponse) {
		return fmt.Errorf("mismatch slice length expected='%v' got='%v'", len(expectedResponse), len(gotResponse))
	}

	for index := range expectedResponse {
		if err := Val(expectedResponse[index], gotResponse[index]); err != nil {
			return err
		}
	}

	return nil
}
