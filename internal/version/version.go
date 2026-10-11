package version

var (
	// Set via ldflags at build time
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
