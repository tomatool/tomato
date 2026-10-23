package command

import "testing"

func TestReportArgs(t *testing.T) {
	for _, tc := range []struct {
		args              []string
		run, format, path string
		wantErr           bool
	}{
		{nil, "", "ctrf", "", false},
		{[]string{"216b61f6"}, "216b61f6", "ctrf", "", false},
		{[]string{"216b61f6", "-f", "junit", "-o", "out.xml"}, "216b61f6", "junit", "out.xml", false},
		{[]string{"--format=junit", "216b61f6", "--output", "r.xml"}, "216b61f6", "junit", "r.xml", false},
		{[]string{"a", "b"}, "", "", "", true},
		{[]string{"a", "-x"}, "", "", "", true},
		{[]string{"a", "-f"}, "", "", "", true},
	} {
		run, format, path, err := reportArgs(tc.args, "ctrf", "")
		if (err != nil) != tc.wantErr || (!tc.wantErr && (run != tc.run || format != tc.format || path != tc.path)) {
			t.Errorf("reportArgs(%q) = %q %q %q %v", tc.args, run, format, path, err)
		}
	}
}
