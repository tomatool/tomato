package cmd

import (
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var InitCmd *cli.Command = &cli.Command{
	Name:  "init",
	Usage: "Initialize tomato testing for a project",
	Action: func(ctx *cli.Context) error {
		return errors.New("not yet implemented")
	},
}
