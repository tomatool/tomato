package server

import "testing"

func TestNormalizePath(t *testing.T) {
	for _, test := range []struct {
		path        string
		jsonizePath string
	}{
		{"/selection?hola=cool", `/selection?{"hola":"cool"}`},
		{"/selection?hola=cool&cool=hola", `/selection?{"cool":"hola","hola":"cool"}`},
		{"/selection?hola=cool&abc=cde&cool=hola", `/selection?{"abc":"cde","cool":"hola","hola":"cool"}`},
		{"/selection?hola=cool&cool=hola&abc=cde", `/selection?{"abc":"cde","cool":"hola","hola":"cool"}`},
		{"/selection?cool=hola&hola=cool&abc=cde", `/selection?{"abc":"cde","cool":"hola","hola":"cool"}`},
	} {
		if o := jsonizePath(test.path); o != test.jsonizePath {
			t.Errorf("expecting %s, got %s", test.jsonizePath, o)
		}
	}
}
