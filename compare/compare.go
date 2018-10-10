package compare

import (
	"fmt"
	"reflect"

	"github.com/olekukonko/tablewriter"
)

// Comparison records the state of a comparison
type Comparison struct {
	output      string
	errorPrefix string
}

// ShouldFailStep satisfies behavior of whether or not a step should fail
func (c Comparison) ShouldFailStep() bool {
	return c.output != ""
}

// Error returns the formatter error string with context and error
func (c Comparison) Error() string {
	return fmt.Sprintf("%s\n\n%s", c.errorPrefix, c.output)
}

func Value(a, b interface{}) error {
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)

	if bb, ok := b.(string); ok {
		if bb == "*" {
			return nil
		}
	}

	if ta != tb {
		return fmt.Errorf("TypeOf: [%v] %v != [%v] %v", ta, a, tb, b)
	}

	if a == nil {
		return nil
	}

	if ta.Kind() == reflect.Map {
		va := reflect.ValueOf(a)
		vb := reflect.ValueOf(b)

		if va.Len() < vb.Len() {
			return fmt.Errorf("SizeOf: [%v] %+v != [%v] %+v", va.Len(), a, vb.Len(), b)
		}

		ma := make(map[interface{}]interface{})
		for _, key := range va.MapKeys() {
			ma[key.Interface()] = va.MapIndex(key).Interface()
		}

		mb := make(map[interface{}]interface{})
		for _, key := range vb.MapKeys() {
			mb[key.Interface()] = vb.MapIndex(key).Interface()
		}

		for key, vb := range mb {
			va, ok := ma[key]
			if !ok {
				return fmt.Errorf("MissingKey: %s from %+v", key, ma)
			}
			if err := Value(va, vb); err != nil {
				return err
			}
		}

		return nil
	}

	if ta.Kind() == reflect.Slice {
		va := reflect.ValueOf(a)
		vb := reflect.ValueOf(b)

		if va.Len() != vb.Len() {
			return fmt.Errorf("SizeOf: [%v] %+v != [%v] %+v", va.Len(), a, vb.Len(), b)
		}

		for i := 0; i < va.Len(); i++ {
			if err := Value(
				va.Index(i).Interface(),
				vb.Index(i).Interface(),
			); err != nil {
				return err
			}
		}
		return nil
	}

	av, ok := a.(string)
	if ok && av == "*" {
		return nil
	}
	bv, ok := b.(string)
	if ok && bv == "*" {
		return nil
	}

	if a != b {
		return fmt.Errorf("Mismatch: %+v != %+v", a, b)
	}
	return nil
}

func Print(t *tablewriter.Table, key string, A interface{}, B interface{}) {
	ta := reflect.TypeOf(A)
	if ta.Kind() == reflect.Slice {
		va := reflect.ValueOf(A)
		vb := reflect.ValueOf(B)

		for i := 0; i < va.Len(); i++ {
			Print(
				t,
				fmt.Sprintf("%d", i),
				va.Index(i).Interface(),
				vb.Index(i).Interface(),
			)
		}
		return
	}
	if ta.Kind() == reflect.Map {
		va := reflect.ValueOf(A)
		vb := reflect.ValueOf(B)

		ma := make(map[interface{}]interface{})
		for _, key := range va.MapKeys() {
			ma[key.Interface()] = va.MapIndex(key).Interface()
		}

		mb := make(map[interface{}]interface{})
		for _, key := range vb.MapKeys() {
			mb[key.Interface()] = vb.MapIndex(key).Interface()
		}

		for key := range ma {
			Print(t, key.(string), ma[key], mb[key])
		}
		return
	}
	t.SetHeader([]string{
		"key", "actual", "expected",
	})
	t.Append([]string{
		fmt.Sprintf("%s", key),
		fmt.Sprintf("%+v", A),
		fmt.Sprintf("%+v", B),
	})
}
