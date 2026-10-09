# tomato ui

The web UI `tomato ui` serves: React, built by Vite with pnpm and embedded into
the Go binary.

## Layout

```
ui/
├── index.html          host page + the inlined resource-icon sprite
└── src/
    ├── main.jsx        mounts <App/>
    ├── App.jsx         state, layout, keyboard, pane dragging
    ├── styles.css      the "Modernist" design system: tokens + components
    ├── components/     TopBar, Tree, ScenarioCard, StepRow, ConfigView,
    │                   Flow, Topology, Runs, Transport, Peek, Console
    ├── hooks/          useSocket (the /ws feed), useLocalNumber (remembered sizes)
    ├── lib/            step tokenising, ANSI → nodes, flow model, formatting, icons
    └── fonts/          Geist and Geist Mono, vendored so the UI works offline
```

Everything renders to React nodes — there is no `dangerouslySetInnerHTML`
anywhere, so step text, log lines and config values cannot inject markup.

The design system's contract still holds: state lives in `data-*`/`aria-*`
attributes and class names never change at runtime, so components spread
`data-status={…}` rather than toggling classes.

The build writes to `../command/ui_assets/dist`, which `command/ui.go` embeds
with `//go:embed all:ui_assets/dist`.

## Working on it

```sh
make ui        # pnpm install --frozen-lockfile && pnpm build
make ui-dev    # Vite dev server with HMR
make ui-check  # fail if the committed bundle is stale
```

`make ui-dev` proxies `/api` and `/ws` to `http://localhost:7788`, so start a
backend next to it:

```sh
tomato ui --port 7788 --no-browser   # in your project, not here
```

## Why the bundle is committed

`command/ui_assets/dist` is checked in. Go's build cannot run pnpm, so without
the committed bundle `go build ./...`, `go install` and `go test ./...` would
all fail for anyone without Node. CI rebuilds it and fails when it differs from
`ui/`, the same arrangement the kafka preset's jar uses.

**So: run `make ui` and commit `command/ui_assets/dist` whenever you change
anything under `ui/`.**

## Design system

The CSS comes from a claude.ai/design project called *Modernist*. Its rules:
dark only, state in `data-*`/`aria-*` attributes with classes static, every
status shape-coded as well as coloured, red reserved for failure, and anything
read out of a file set in mono. Changes to the look belong there first.
