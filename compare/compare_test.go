package compare

import "testing"

func TestValue(t *testing.T) {
	for idx, test := range []struct {
		a   interface{}
		b   interface{}
		out bool
	}{
		{nil, nil, true},
		{"", "", true},
		{1, 1, true},
		{1.1, 1.1, true},

		{nil, 0, false},
		{nil, "", false},
		{0, 0.0, false},

		{[]int{}, []int{}, true},
		{[]int{1, 2}, []int{1, 2}, true},
		{[]float64{1, 2}, []float64{1.0, 2.0}, true},
		{[]string{"bob", "alice"}, []string{"bob", "alice"}, true},
		{[]interface{}{"bob", "alice"}, []interface{}{"bob", "alice"}, true},
		{[]interface{}{[]string{"bob", "alice"}, "alice"}, []interface{}{[]string{"bob", "alice"}, "alice"}, true},

		{[]interface{}{[]string{"bob", "alice"}, "alice"}, []interface{}{[]string{"bob", "practice"}, "alice"}, false},
		{[]float64{1, 2}, []int{1, 2}, false},
		{[]int{2, 1}, []int{1, 2}, false},

		{map[string]interface{}{}, map[string]interface{}{}, true},
		{map[string]interface{}{"name": []interface{}{[]string{"bob", "alice"}, "alice"}}, map[string]interface{}{"name": []interface{}{[]string{"bob", "alice"}, "alice"}}, true},
		{[]map[string]interface{}{{"name": []interface{}{[]string{"bob", "alice"}, "alice"}}}, []map[string]interface{}{{"name": []interface{}{[]string{"bob", "alice"}, "alice"}}}, true},

		{map[string]interface{}{"name": []interface{}{[]string{"bob", "aice"}, "alice"}}, map[string]interface{}{"name": []interface{}{[]string{"bob", "alice"}, "alice"}}, false},

		{map[string]string{"name": "joni"}, map[string]string{}, true},
		{map[string]string{"name": "joni"}, map[string]string{"name": "joni"}, true},
		{map[string]string{}, map[string]string{"name": "joni"}, false},
	} {
		err := Value(test.a, test.b)
		if err == nil && test.out == false {
			t.Errorf("%d - expecting %v, got %v", idx, test.out, err)
		}
		if err != nil && test.out == true {
			t.Errorf("%d - expecting %v, got %v", idx, test.out, err)
		}

	}
}
