package cmd

import "testing"

func TestGetDirPath(t *testing.T) {
	if d := getDirPath("hello/world/123.html"); d != "hello/world" {
		t.Fatal("expecting getdirpath to return hello/world, got", d)
	}
}
