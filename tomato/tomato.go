package tomato

import (
	"log"
	"os"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/pkg/errors"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/formatter"
	"github.com/tomatool/tomato/handler"
	"github.com/tomatool/tomato/resource"
)

func init() {
	// register tomato custom formatter
	godog.Format("tomato", "tomato custom godog formatter", formatter.New)
}

type Tomato struct {
	log    *log.Logger
	config *config.Config

	readinessTimeout time.Duration
}

func New(conf *config.Config, logger *log.Logger) *Tomato {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}

	return &Tomato{
		config: conf,
		log:    logger,
	}
}

func (t *Tomato) Verify() error {
	if len(t.config.FeaturesPaths) == 0 {
		return errors.New("Features path is missing, please specify on config file or flag\nFor additional help try 'tomato -help'")
	}

	return nil
}

func (t *Tomato) Run() error {
	t.log.Println("🍅 testing suite starting...")
	opts := godog.Options{
		Output:        colors.Colored(os.Stdout),
		Paths:         t.config.FeaturesPaths,
		Format:        "tomato",
		Strict:        true,
		StopOnFailure: t.config.StopOnFailure,
	}

	if t.config.Randomize {
		opts.Randomize = time.Now().UTC().UnixNano()
	}

	if t.config.ReadinessTimeout == "" {
		t.config.ReadinessTimeout = "15s"
	}
	readinessTimeout, err := time.ParseDuration(t.config.ReadinessTimeout)
	if err != nil {
		t.log.Printf(colors.Yellow("Failed to parse duration of %s: %v"), t.config.ReadinessTimeout, err)
		readinessTimeout = time.Second * 15
	}
	t.readinessTimeout = readinessTimeout

	t.log.Printf("Configuration:\n  Features\t\t: %s\n  Randomize\t\t: %v\n  Stop on Failure\t: %v\n  Readiness Timeout\t: %s\n", t.config.FeaturesPaths, t.config.Randomize, t.config.StopOnFailure, t.readinessTimeout.String())

	h := handler.New()

	t.log.Printf("Resources Readiness:\n")
	for _, cfg := range t.config.Resources {
		resource, err := handler.CreateResource(cfg)
		if err != nil {
			return errors.Wrapf(err, "  [%s] Error", cfg.Name)
		}

		h.Register(cfg, resource)
		if v, ok := cfg.Options["readiness_check"]; ok && v != "true" {
			t.log.Printf("  [%s] Skipping\n", cfg.Name)
			continue
		}
		if err := t.waitResource(resource); err != nil {
			return errors.Wrapf(err, "  [%s] Error", cfg.Name)
		}

		t.log.Printf("  [%s] Ready\n", cfg.Name)
	}

	t.log.Printf("Test Result:\n")
	if result := godog.RunWithOptions("godogs", h.Handler(), opts); result != 0 {
		return errors.New("Test failed")
	}

	return nil
}

func (t *Tomato) waitResource(r resource.Resource) error {
	var (
		err  error
		done = make(chan struct{})
	)

	ticker := time.NewTicker(time.Millisecond * 300)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			err = r.Open()
			if err != nil {
				continue
			}
			err = r.Ready()
			if err != nil {
				continue
			}
			done <- struct{}{}
			break
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second * 15):
		return errors.Wrap(err, "timeout after 15s")
	}

	return err
}
