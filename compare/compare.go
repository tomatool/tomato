package compare

import (
	"fmt"
	"reflect"

	"github.com/olekukonko/tablewriter"
)

func Value(a, b interface{}) bool {
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)

	if ta != tb {
		return false
	}

	if a == nil {
		return true
	}

	if ta.Kind() == reflect.Map {
		va := reflect.ValueOf(a)
		vb := reflect.ValueOf(b)

		if va.Len() != vb.Len() {
			return false
		}

		ma := make(map[interface{}]interface{})
		for _, key := range va.MapKeys() {
			ma[key.Interface()] = va.MapIndex(key).Interface()
		}

		mb := make(map[interface{}]interface{})
		for _, key := range vb.MapKeys() {
			mb[key.Interface()] = vb.MapIndex(key).Interface()
		}

		for key, va := range ma {
			vb, ok := mb[key]
			if !ok {
				return false
			}
			if !Value(va, vb) {
				return false
			}
		}

		return true
	}

	if ta.Kind() == reflect.Slice {
		va := reflect.ValueOf(a)
		vb := reflect.ValueOf(b)

		if va.Len() != vb.Len() {
			return false
		}

		for i := 0; i < va.Len(); i++ {
			if !Value(
				va.Index(i).Interface(),
				vb.Index(i).Interface(),
			) {
				return false
			}
		}
		return true
	}

	av, ok := a.(string)
	if ok && av == "*" {
		return true
	}
	bv, ok := b.(string)
	if ok && bv == "*" {
		return true
	}

	if a != b {
		return false
	}
	return true
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
