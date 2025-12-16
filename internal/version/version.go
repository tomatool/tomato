package version

import "runtime"

var (
	// Set via ldflags at build time
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info returns version information
func Info() map[string]string {
	return map[string]string{
		"version":   Version,
		"commit":    Commit,
		"built":     BuildDate,
		"go":        runtime.Version(),
		"os/arch":   runtime.GOOS + "/" + runtime.GOARCH,
	}
}
