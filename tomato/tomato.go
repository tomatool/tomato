package tomato

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/tomatool/tomato/composer"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/formatter"
	"github.com/tomatool/tomato/handler"
	"github.com/tomatool/tomato/resource"

	httpclient_r "github.com/tomatool/tomato/resource/httpclient"
	mysql_r "github.com/tomatool/tomato/resource/mysql"
	nsq_r "github.com/tomatool/tomato/resource/nsq"
	postgres_r "github.com/tomatool/tomato/resource/postgres"
	rabbitmq_r "github.com/tomatool/tomato/resource/rabbitmq"
	redis_r "github.com/tomatool/tomato/resource/redis"
	shell_r "github.com/tomatool/tomato/resource/shell"
	wiremock_r "github.com/tomatool/tomato/resource/wiremock"
)

func init() {
	// register tomato custom formatter
	godog.Format("tomato", "tomato custom godog formatter", formatter.New)
}

type Tomato struct {
	log    *log.Logger
	config *config.Config

	composer composer.Composer

	readinessTimeout time.Duration
}

func NewTomato(config *config.Config) (t *Tomato) {
	t = &Tomato{
		config: config,
	}

	if t.log == nil {
		t.log = log.New(os.Stdout, "", 0)
	}

	switch config.Composer {
	case composer.ComposerTypeDocker:
		client, err := client.NewEnvClient()
		if err != nil {
			panic("Failed to create docker client: " + err.Error())
		}
		t.composer = &composer.DockerClient{
			Client: client,
		}
	case composer.ComposerTypeKubernetes:
	default:
		t.composer = &composer.DefaultComposer{}
	}

	return t
}

func (t *Tomato) Run(ctx context.Context) error {
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

	for _, resourceConfig := range t.config.ResourcesConfig {
		err := t.composer.CreateContainer(ctx, resourceConfig)
		if err != nil {
			return err
		}

		resourceHandler, err := handler.CreateResourceHandler(resourceConfig)
		if err != nil {
			return errors.Wrapf(err, "  [%s] Error", resourceConfig.Name)
		}
		h.Register(resourceConfig, resourceHandler)

		if v, ok := resourceConfig.Params["readiness_check"]; ok && v != "true" {
			t.log.Printf("  [%s] Skipping\n", resourceConfig.Name)
			continue
		}
		if err := t.waitResource(resourceHandler); err != nil {
			return errors.Wrapf(err, "  [%s] Error", resourceConfig.Name)
		}

		t.log.Printf("  [%s] Ready\n", resourceConfig.Name)
	}

	t.log.Printf("Test Result:\n")
	if result := godog.RunWithOptions("godogs", h.Handler(), opts); result != 0 {
		return errors.New("Test failed")
	}

	return nil
}

func (t *Tomato) Close(ctx context.Context) error {
	switch t.config.Composer {
	case composer.ComposerTypeDocker:
		return t.composer.DeleteAll(ctx)
	case composer.ComposerTypeKubernetes:
		return t.composer.DeleteAll(ctx)
	default:
	}
	return nil
}

func (t *Tomato) CreateResourceHandler(cfg *config.Resource) (resource.Handler, error) {
	switch cfg.Type {
	case "mysql":
		return mysql_r.New(cfg)
	case "postgres":
		return postgres_r.New(cfg)
	case "rabbitmq":
		return rabbitmq_r.New(cfg)
	case "nsq":
		return nsq_r.New(cfg)
	case "httpclient":
		return httpclient_r.New(cfg)
	case "wiremock":
		return wiremock_r.New(cfg)
	case "shell":
		return shell_r.New(cfg)
	case "redis":
		return redis_r.New(cfg)
	}
	return nil, fmt.Errorf("resource type `%s` is not defined\nplease refer to %s for list of available resources",
		cfg.Type,
		colors.Bold(colors.White)("https://github.com/tomatool/tomato#resources"))
}

func (t *Tomato) waitResource(r resource.Handler) error {
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
