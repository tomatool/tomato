package cmp

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
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

	if err := compareMap(mapA, mapB); err != nil {
		logrus.WithFields(logrus.Fields{
			"Error":     err,
			"Received:": string(a),
			"Expected:": string(b),
		}).Errorf("Unexpected response")
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", string(a), string(b), err.Error())
	}

	if exact {
		if err := compareMap(mapB, mapA); err != nil {
			logrus.WithFields(logrus.Fields{
				"Error":     err,
				"Received:": string(a),
				"Expected:": string(b),
			}).Errorf("Unexpected response")
			return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", string(a), string(b), err.Error())
		}
	}

	return nil
}

func compareMap(expectedResponse, gotResponse map[string]interface{}) error {
	for key := range expectedResponse {
		expectedVal, ok1 := expectedResponse[key]
		gotVal, ok2 := gotResponse[key]
		if ok1 != ok2 {
			logrus.WithFields(logrus.Fields{
				"Received:": ok1,
				"Expected:": ok2,
			}).Errorf("Mistmatched field key: %s", key)
			return fmt.Errorf("mismatch field key='%s' expected='%v' got='%v'", key, ok1, ok2)
		}

		if err := compareVal(expectedVal, gotVal); err != nil {
			logrus.WithFields(logrus.Fields{
				"Error:": err,
			}).Errorf("Mistmatched field key: %s", key)
			return fmt.Errorf("[%s] %s", key, err.Error())
		}
	}
	return nil
}

func compareVal(expectedVal, gotVal interface{}) error {
	expectedType := reflect.TypeOf(expectedVal)
	if expectedType != nil &&
		expectedType.Kind() == reflect.String &&
		expectedVal.(string) == "*" {
		return nil
	}

	gotType := reflect.TypeOf(gotVal)
	if expectedType != gotType {
		logrus.WithFields(logrus.Fields{
			"Received:": gotType,
			"Expected:": expectedType,
		}).Errorf("Mistmatched value type")
		return fmt.Errorf("mismatch value type expected='%v' got='%v'", expectedType, gotType)
	}
	if expectedType == nil {
		return nil
	}

	if expectedType.Kind() == reflect.Slice {
		if err := compareSlice(
			expectedVal.([]interface{}),
			gotVal.([]interface{}),
		); err != nil {
			return err
		}
		return nil
	}

	if expectedType.Kind() == reflect.Map {
		if err := compareMap(
			expectedVal.(map[string]interface{}),
			gotVal.(map[string]interface{}),
		); err != nil {
			return err
		}
		return nil
	}

	if expectedVal != gotVal {
		logrus.WithFields(logrus.Fields{
			"Received:": gotVal,
			"Expected:": expectedVal,
		}).Errorf("Mistmatched value")
		return fmt.Errorf("mismatch value expected='%v' got='%v'", expectedVal, gotVal)
	}
	return nil
}

func compareSlice(expectedResponse, gotResponse []interface{}) error {
	if len(expectedResponse) != len(gotResponse) {
		logrus.WithFields(logrus.Fields{
			"Received:": len(gotResponse),
			"Expected:": len(gotResponse),
		}).Errorf("Mistmatched slice length")
		return fmt.Errorf("mismatch slice length expected='%v' got='%v'", len(expectedResponse), len(gotResponse))
	}

	for index := range expectedResponse {
		if err := compareVal(expectedResponse[index], gotResponse[index]); err != nil {
			return err
		}
	}

	return nil
}
