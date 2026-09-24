package handler

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestParseCode(t *testing.T) {
	cases := map[string]codes.Code{
		"OK":                  codes.OK,
		"NOT_FOUND":           codes.NotFound,
		"not_found":           codes.NotFound,
		"NotFound":            codes.NotFound,
		"FAILED_PRECONDITION": codes.FailedPrecondition,
		"Unavailable":         codes.Unavailable,
		"UNAUTHENTICATED":     codes.Unauthenticated,
		"5":                   codes.NotFound,
	}
	for in, want := range cases {
		got, err := parseCode(in)
		if err != nil || got != want {
			t.Errorf("parseCode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseCode("NOPE"); err == nil {
		t.Error("unknown status should fail")
	}
}
