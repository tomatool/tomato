package main

import (
	"fmt"
	"os"
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
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
