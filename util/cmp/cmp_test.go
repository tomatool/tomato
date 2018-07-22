package cmp_test

import (
	"strings"
	"testing"

	"github.com/alileza/tomato/util/cmp"
)

func TestJSON(t *testing.T) {
	for idx, test := range []struct {
		a     string
		b     string
		exact bool

		err string
	}{
		{"", "", false, "unexpected end of JSON input"},
		{"", "", true, "unexpected end of JSON input"},
		{"{}", "", false, "unexpected end of JSON input"},
		{"{}", "", true, "unexpected end of JSON input"},
		{`{"num":[1,2,3]}`, `{"num":[1,3]}`, false, "mismatch slice length"},
		{`{"num":[1,2,3]}`, `{"num":[1,2]}`, true, "mismatch slice length"},
		{`{"num":[1,3]}`, `{"num":[1,2]}`, false, "mismatch value expected"},
		{`{"num":[1,3]}`, `{"num":[1,2]}`, true, "mismatch value expected"},
		{"{}", "{}", false, ""},
		{`{"name":{"age":"*"}}`, `{"name":{"age":{"name":123}}}`, false, ""},
		{`{"name":{"age":"*"}}`, `{"name":{"age":{"name":123}}}`, true, "mismatch value type expected"},
		{`{"test":1}`, `{}`, false, "mismatch field key='test'"},
		{`{"test":1}`, `{}`, true, "mismatch field key='test'"},
		{`{}`, `{"test":1}`, false, ""},
		{`{}`, `{"test":1}`, true, "mismatch field key='test'"},
		{`{"test":1}`, `{"test":2}`, false, "mismatch value expected='1'"},
		{`{"test":1}`, `{"test":2}`, true, "mismatch value expected='1'"},
		{`{"test":2}`, `{"test":1}`, false, "mismatch value expected='2'"},
		{`{"test":2}`, `{"test":1}`, true, "mismatch value expected='2'"},
		{`{"test":{"abc":"cde"}}`, `{"test":{"abc":"cde"}}`, false, ""},
		{`{"test":{"abc":"cde"}}`, `{"test":{"abc":"cde"}}`, true, ""},
		{`{"test":"*"}`, `{"test":{"abc":"cde"}}`, false, ""},
		{`{"test":"*"}`, `{"test":{"abc":"cde"}}`, true, "mismatch value type expected="},
		{`{"test":{"abc":"cde"}}`, `{"test":"*"}`, false, "mismatch value type expected="},
		{`{"test":{"abc":"cde"}}`, `{"test":"*"}`, true, "mismatch value type expected="},
	} {
		err := cmp.JSON([]byte(test.a), []byte(test.b), test.exact)
		if err == nil && test.err == "" {
			continue
		}
		if err == nil && test.err != "" {
			t.Fatalf("%d - expecting err, got=nil", idx)
		}

		if test.err == "" && err != nil {
			t.Errorf("%d - expecting err to be nil, got=%v", idx, err)
		}
		if test.err != "" && !strings.Contains(err.Error(), test.err) {
			t.Errorf("%d - expecting err to contains %s, got=%v", idx, test.err, err)
		}
	}
}
