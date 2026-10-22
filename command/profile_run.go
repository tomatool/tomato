package command

import (
	"context"
	"encoding/json"
	"os"

	"github.com/tomatool/tomato/internal/apprunner"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/profile"
	"github.com/tomatool/tomato/internal/registry"
)

// newProfiler measures the app and every container tomato started for it.
func newProfiler(cfg *config.Config, cm *container.Manager, reg *registry.Registry, app *apprunner.Runner) *profile.Profiler {
	src := profile.Sources{
		Containers: cm.Names(),
		Stats:      dockerStats{cm},
		Meters:     reg.Meters(),
	}
	if app.GetMode() == apprunner.ModeContainer {
		src.AppContainer = cfg.App.GetName()
	} else {
		src.AppProcessGroup = app.ProcessGroup
	}
	return profile.New(src)
}

// dockerStats streams a container's stats from the container manager.
type dockerStats struct{ cm *container.Manager }

func (d dockerStats) StreamStats(ctx context.Context, name string, fn func(profile.StatsReading)) error {
	return d.cm.StreamStats(ctx, name, func(s container.Stats) {
		fn(profile.StatsReading{At: s.At, CPU: s.CPU, Memory: s.Memory})
	})
}

// writeProfile saves the run's profile next to its logs.
func writeProfile(path string, r profile.Result) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
