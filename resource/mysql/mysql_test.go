package mysql

import "testing"

func TestGetDatabaseName(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{
		{
			"",
			"",
		},
		{
			"root:potato@tcp(mysql:3306)/tomato",
			"tomato",
		},
		{
			"root:potato@tcp(mysql:3306)/potato?uyeah",
			"potato",
		},
		{
			"mysql://root:potato@tcp(mysql:3306)/toma-to?uyeah",
			"toma-to",
		},
	}
	for _, tc := range testCases {
		if out := getDatabaseName(tc.input); out != tc.output {
			t.Errorf("Unexpected %s, expecting %s", out, tc.output)
		}
	}
}
