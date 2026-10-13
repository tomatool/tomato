package registry

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

func TestInitAppEnvProviders_InitializesOnlyProvidersAndMergesTheirEnv(t *testing.T) {
	registry, err := New(map[string]config.Resource{
		"aws": {Type: "aws", Options: map[string]any{"region": "eu-central-1"}},
	}, nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = registry.Cleanup(context.Background()) })

	// A resource that provides nothing to the app's environment must not be
	// touched yet: it may well need the app to be running first.
	plain := &countingHandler{}
	registry.handlers["plain"] = plain

	env, err := registry.InitAppEnvProviders(context.Background())
	if err != nil {
		t.Fatalf("InitAppEnvProviders: %v", err)
	}
	if env.Set["AWS_REGION"] != "eu-central-1" || !strings.HasPrefix(env.Set["AWS_ENDPOINT_URL_STS"], "http://127.0.0.1:") {
		t.Errorf("provided env %v", env.Set)
	}
	if len(env.Unset) == 0 {
		t.Error("no inherited variables to remove")
	}

	if plain.inits != 0 {
		t.Error("a resource that provides no app environment was initialized before the app")
	}

	// The runner initializes every resource again once the app is up.
	if err := registry.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady after InitAppEnvProviders: %v", err)
	}
	if plain.inits != 1 {
		t.Errorf("resource initialized %d times once the app was up, want once", plain.inits)
	}
}

// countingHandler counts the lifecycle calls the registry makes.
type countingHandler struct{ inits, cleanups int }

func (h *countingHandler) Name() string                         { return "counting" }
func (h *countingHandler) Init(context.Context) error           { h.inits++; return nil }
func (h *countingHandler) Ready(context.Context) error          { return nil }
func (h *countingHandler) Reset(context.Context) error          { return nil }
func (h *countingHandler) RegisterSteps(*godog.ScenarioContext) {}
func (h *countingHandler) Cleanup(context.Context) error        { h.cleanups++; return nil }

// Nothing called Cleanup at the end of a run: the aws resource's token and
// credential files stayed behind, and clients stayed connected while their
// containers stopped. It runs once, since some clients panic on a second close.
func TestRegistryCleanup_ReleasesResourcesOnce(t *testing.T) {
	registry, err := New(map[string]config.Resource{
		"aws": {Type: "aws"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	counting := &countingHandler{}
	registry.handlers["counting"] = counting

	env, err := registry.InitAppEnvProviders(context.Background())
	if err != nil {
		t.Fatalf("InitAppEnvProviders: %v", err)
	}
	tokenFile := env.Set["AWS_WEB_IDENTITY_TOKEN_FILE"]
	if _, err := os.Stat(tokenFile); err != nil {
		t.Fatalf("no token file before cleanup: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := registry.Cleanup(context.Background()); err != nil {
			t.Fatalf("Cleanup %d: %v", i+1, err)
		}
	}
	if counting.cleanups != 1 {
		t.Errorf("handler cleaned up %d times, want once", counting.cleanups)
	}
	if _, err := os.Stat(filepath.Dir(tokenFile)); !os.IsNotExist(err) {
		t.Errorf("the aws resource's files are still there: %v", err)
	}
	if _, err := http.Get(env.Set["AWS_ENDPOINT_URL_STS"]); err == nil {
		t.Error("the STS still answers after cleanup")
	}
}

// A provider that provides nothing in this configuration (a kafka resource
// without a Glue registry) is initialized with everything else, not before the
// app: early, it could reach for containers or an app that are not there yet.
func TestInitAppEnvProviders_SkipsConditionalProvidersThatProvideNothing(t *testing.T) {
	registry, err := New(map[string]config.Resource{}, nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	off := &conditionalHandler{provides: false}
	on := &conditionalHandler{provides: true}
	registry.handlers["off"] = off
	registry.handlers["on"] = on

	env, err := registry.InitAppEnvProviders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if off.inits != 0 {
		t.Error("a conditional provider that provides nothing was initialized before the app")
	}
	if on.inits != 1 || env.Set["CONDITIONAL"] != "yes" {
		t.Errorf("a conditional provider that provides was skipped: inits %d, env %v", on.inits, env.Set)
	}
}

type conditionalHandler struct {
	countingHandler
	provides bool
}

func (h *conditionalHandler) ProvidesAppEnv() bool { return h.provides }
func (h *conditionalHandler) AppEnv() resource.AppEnv {
	return resource.AppEnv{Set: map[string]string{"CONDITIONAL": "yes"}}
}

// The lifecycle resource restarts the containers a scenario stopped before
// any other resource resets: a postgres reset against a stopped container
// would fail. And it is handed the app once the app runs.
func TestRegistry_ResetsFirstAndAppControl(t *testing.T) {
	registry, err := New(map[string]config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, name := range []string{"a", "b", "c", "d"} {
		registry.handlers[name] = &orderedHandler{name: name, order: &order}
	}
	first := &firstHandler{orderedHandler: orderedHandler{name: "lifecycle", order: &order}}
	registry.handlers["lifecycle"] = first

	if err := registry.ResetAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if order[0] != "lifecycle" || len(order) != 5 {
		t.Errorf("reset order %v: lifecycle must be first and every handler reset once", order)
	}

	registry.SetAppControl(fakeAppControl{})
	if first.app == nil {
		t.Error("an AppControlled resource was not handed the app")
	}
}

type orderedHandler struct {
	countingHandler
	name  string
	order *[]string
}

func (h *orderedHandler) Reset(context.Context) error {
	*h.order = append(*h.order, h.name)
	return nil
}

type firstHandler struct {
	orderedHandler
	app resource.AppControl
}

func (h *firstHandler) ResetsFirst() bool                     { return true }
func (h *firstHandler) SetAppControl(app resource.AppControl) { h.app = app }

type fakeAppControl struct{}

func (fakeAppControl) StopApp() error                                    { return nil }
func (fakeAppControl) StartApp(context.Context, map[string]string) error { return nil }
func (fakeAppControl) Running() bool                                     { return true }
