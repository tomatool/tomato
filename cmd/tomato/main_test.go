// +build testmain

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

func init() {
	flag.String("config.file", "", "")
	flag.String("features.path", "", "")
}

func TestMain(t *testing.T) {
	var args []string
	for _, arg := range os.Args {
		if !strings.HasPrefix(arg, "-test") {
			args = append(args, arg)
		}
	}
	os.Args = args
	main()
	fmt.Println("application exit gracefully 👼")
}
