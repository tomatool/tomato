package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/command"
)

func main() {
	// Configure pretty console output by default
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.Kitchen,
		NoColor:    false,
	}).With().Timestamp().Logger()

	// Default to info level (hide debug logs)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if err := command.Run(os.Args); err != nil {
		// godog has already printed failing scenarios; everything else (a
		// resource that can't connect, a failing hook) would otherwise exit 1
		// with no explanation.
		if !strings.HasPrefix(err.Error(), "tests failed") {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		}
		os.Exit(1)
	}
}
