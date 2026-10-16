# Browser configuration

The `browser` resource uses your application's web UI the way a person does,
in a real headless Chromium: it opens pages, clicks and types into what it
finds **by the name a person reads** — a button's label, a field's label, a
paragraph's text — and asserts on what the page **shows**. The `http`
resource sees the API; this one sees whether the UI shows the right thing and
sends the right call when someone clicks.

```yaml
version: 2

containers:
  chrome:
    preset: chromium

app:
  command: go run ./cmd/server
  port: 8080

resources:
  alice:
    type: browser
    container: chrome
    options:
      base_url: http://localhost:8080
      viewport: 1440x900
      locale: de-DE
  bob:
    type: browser
    container: chrome
    options:
      base_url: http://localhost:8080
```

```gherkin
Scenario: Two editors see each other's changes
  Given "alice" opens "/master"
  And "bob" opens "/master"
  When "alice" types " Neuer Satz." at the end of "7.2"
  Then "bob" sees "Neuer Satz." within "3s"
```

Each resource is its own browser context — its own cookies, local and session
storage and service workers — in one shared Chromium, so two resources are two
people. Collaboration needs no special syntax.

## Options

| Option | Default | Meaning |
|--------|---------|---------|
| `base_url` | none | Where a path in `opens "/master"` is opened. `127.0.0.1` is read as `localhost`. Also accepted at the resource's top level, as for `http`. |
| `viewport` | `1280x720` | The window size, `WIDTHxHEIGHT`. |
| `locale` | Chromium's | The language the page sees: `navigator.language`, `Intl` formatting and the `Accept-Language` header of every request. |
| `screenshots` | `.tomato/runs/<run>/screenshots` | Where screenshots are written. The default sits beside the run's logs, which CI keeps. |
| `timeout` | `5s` | How long a step waits for the element it names to appear, become enabled and stop being covered. |
| `port` | `9222` | The DevTools port inside the container, for a container that is not the preset. |

## How a step finds what it names

`"alice" clicks "Apply"` does not take a CSS selector. tomato asks Chromium's
accessibility tree for elements whose **accessible name** is `Apply` — the
text of a button or link, the `<label>` of a field, `aria-label`,
`aria-labelledby`, a placeholder — the same name a screen reader announces.
When nothing has that name, the element whose **visible text** is exactly
`Apply` is used. Only visible elements count.

- **Exactly one must match.** When a button and a heading are both called
  `Apply`, the button wins: a person clicking `Apply` means the control. Two
  buttons called `Delete` fail the step and list both, rather than clicking
  whichever came first.
- **Every step waits** up to `timeout` for the element to be there, enabled
  and not covered by something else (a modal's backdrop, a toast). After a
  click everything is asynchronous, so a step that fails on the first look
  would be a flaky step.
- **A click lands where it was aimed, or not at all.** When the page moves
  between finding the element and the click arriving — a "Saving…" line
  appears above it, a banner slides in — the click would hit whatever moved
  under the pointer. tomato stops that click before the page sees it and
  tries again.
- **A label stands for its control.** `checks "Austria"` finds the checkbox
  whether the text is in a `<label>` around it, a `<label for>` beside it or
  an `aria-label` on it. A checkbox styled away behind its label is clicked
  through the label, as a person would.
- **`types "…" at the end of "7.2"`** finds the paragraph that shows `7.2`
  (the text may be part of a longer line), puts the caret after its last
  character, and types key by key: `keydown`, `beforeinput`, `input`, `keyup`.
  Rich-text editors such as ProseMirror and Tiptap see a person typing.
  `fills` replaces a field's content in one edit, as a paste would.

`sees` reads the page's rendered text (`innerText`) and the values of visible
fields, with runs of whitespace as one space. Text in a hidden element, an
attribute or the page's source is not seen, so `does not see "clause_id"`
proves a structure the user must not see stays out of view.

`sees` and `does not see` assert the page **now**. After an action, wait for
its outcome with `sees "…" within "5s"`, or for something to go away with
`no longer sees "…" within "5s"`.

## The chromium preset

```yaml
containers:
  chrome:
    preset: chromium
```

The preset runs `chromedp/headless-shell` (a pinned version), serves the
DevTools protocol on port `9222`, and waits until Chromium answers.

**Reaching the app.** The app runs on your machine, and inside a container
`localhost` is the container. tomato does not rewrite URLs to another host
name: that would change the page's origin, which breaks cookies, CORS, the
absolute redirects an app builds, and the secure context browsers grant only
to `localhost` over plain `http` (where `crypto.randomUUID` and service
workers live). Instead, Chromium resolves the name `localhost` to your
machine, through a tunnel testcontainers opens for each port the suite uses
there: `app.port`, and every `localhost` port named by a `browser`, `http` or
`websocket` resource's URL and by an `http-server` or `websocket-server`
resource's `port`. The tunnel is an SSH connection from tomato to a small sidecar
container (`testcontainers/sshd`), so it does not depend on
`host.docker.internal`, which Docker on Linux and Lima do not point at the
host.

Containers on tomato's network are reached by their name, as from any other
container: `base_url: http://frontend:3000`.

Anything the entry sets as `command` is added to Chromium's flags:

```yaml
containers:
  chrome:
    preset: chromium
    command: ["--force-prefers-reduced-motion"]
```

The preset's entrypoint relies on the layout of `chromedp/headless-shell`, so
an `image:` override has to be another version of that image. To run a
different browser image, leave the preset out and set `image`, `ports:
["9222/tcp"]` and `wait_for` yourself; `localhost` then means that container.

## Between scenarios

Reset closes each person's browser context: its pages, cookies, storage and
service workers. Chromium and its container keep running — a new context is
cheap and a new browser is not — and the next step that needs a page opens a
fresh one.

## Screenshots

When a step fails, each person with an open page leaves a screenshot of what
the page showed, named after the scenario and the resource, and the run's log
says where it is:

```
INF screenshot of the page when the step failed path=.tomato/runs/…/screenshots/failed-Clause-structure-alice.png
```

It is attached to the failed step in the `cucumber` report too. To keep one
from a passing scenario, `"alice" saves screenshot "after-apply"` writes
`after-apply.png`.

## Dialogs

`alert()`, `confirm()` and `prompt()` block a page until someone answers. The
browser resource answers OK, as a person clicking through the flow would, and
logs the dialog's message.

## Limits

- Elements inside an `<iframe>` are not found, and the visible-text fallback
  does not look inside shadow roots (the accessibility tree does).
- A link that opens a new tab or window is not followed into it.
- File uploads, downloads and drag and drop have no steps yet.
- There is no step to narrow a lookup to part of the page ("Delete" in the
  row of order 42); give controls distinct accessible names, which helps
  screen-reader users as well.
