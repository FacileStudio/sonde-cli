# sonde-cli

Terminal client for a Sonde uptime-monitoring instance. Go, cobra, one binary
named `sonde`.

## Commands

| Task | Command |
|---|---|
| Build | `mise run build` |
| Quality gate | `mise run check` |
| Format Go | `mise run format` |
| Architecture and style | `filet check .` |

## Structure

```
main.go                  hands off to cmd
cmd/                     one file per command; root.go owns the global flags,
                         the exit codes and the client factory
internal/
  api/                   the HTTP surface (client.go, auth.go, monitors.go, keys.go)
  config/                the instance URL and the session token
  devicegrant/           RFC 8628 against the identity provider, in three files:
                         constants and transport, discovery.go, poll.go
  loopback/              the same-machine browser flow
  ui/                    CLI-STANDARD §7 output vocabulary
install.sh               a shim; the installer itself is `facile`
```

Dependencies are cobra, `fatih/color`, `golang.org/x/term` and `gopkg.in/yaml.v3`.
Adding a fifth needs a reason: a client for one API does not need a framework.

## Conventions

These come from `~/.mycelium/memory/standards/cli.md`, which is normative and
synced to every machine by mycelium. When this repo disagrees with it, this repo
is wrong.

- **`Short` and flag help: capitalized, imperative, no trailing period.**
- **No emoji, anywhere.** Not in help, not at runtime.
- **All output through `internal/ui`.** `▸` step, `✓` success, `!` warning, `✗`
  error, hints indented two spaces. Warnings and errors go to stderr.
- **`--json` on every command carrying data**, printing one document and nothing
  else. It forces colour off.
- **Exit codes**: `0` success, `1` failure, `2` usage, `130` SIGINT. `root.go`
  maps them; `commandStarted` distinguishes a usage error from a failed one,
  because cobra validates args before its hooks run and flags after them.
- **`--version` prints exactly `sonde <semver>`**: the installer parses that
  line, so `SetVersionTemplate` is not decoration.
- **A credential change is a paired change** with the `sonde` row in
  `facile/internal/manifest/tools.yml`. That row is transcribed from the read
  path here; one wrong character and `facile login sonde` writes a token this
  CLI will never find.

## Traps worth not rediscovering

- **`push` is the one unauthenticated route.** Its credential is the monitor's
  own push token in the path, so `newClient(false)` is deliberate. Everything
  else needs a session.
- **The instance and the identity provider are different machines.** A device
  login can fail because the provider declines the grant, because the instance
  never shipped the exchange, or because neither could be reached. The first two
  fall back to the loopback flow; the third fails, because the machine that
  needs the device grant is the machine that cannot run a browser.
- **`ServesDeviceExchange` probes with an empty POST body.** A route that exists
  refuses it on its merits with a 400; only a 404 says the route is absent.
- **Nothing in a `ui.Table` cell is coloured.** tabwriter measures a cell in
  bytes, so an escape sequence widens the column by characters the terminal
  never draws.
- **`sonde status` has no single endpoint behind it.** The monitors list carries
  no current state, so downness comes from the still-open incidents and the
  percentage from each monitor's uptime window, one call per monitor.
