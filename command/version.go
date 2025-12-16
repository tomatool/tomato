package command

import (
	"fmt"
	"runtime"

	"github.com/tomatool/tomato/internal/version"
	"github.com/urfave/cli/v2"
)

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: "Print version information",
	Action: func(c *cli.Context) error {
		fmt.Printf("tomato version %s\n", version.Version)
		fmt.Printf("  Commit:     %s\n", version.Commit)
		fmt.Printf("  Built:      %s\n", version.BuildDate)
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
