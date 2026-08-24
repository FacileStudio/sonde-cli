# sonde-cli

Terminal client for Sonde, the Facile suite's self-hosted uptime monitor. Pushes heartbeats from cron jobs and manages monitors from the shell.

## What it does

- `sonde push <token>` — cron-friendly heartbeat for push monitors, no login needed
- Browser SSO login through porte with a device-flow fallback for headless machines
- `sonde monitors list/add/remove` against the instance API
- `sonde status` and `sonde incidents` for a quick health readout
- Credential precedence flag > environment (`SONDE_TOKEN`, `SONDE_SERVER_URL`) > config file
- Config at `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`, created owner-only

## Stack

- Go 1.26
- cobra for the command surface
- `fatih/color` for output glyphs per the CLI standard
- GoReleaser for releases (planned)

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/FacileStudio/sonde-cli/main/install.sh | bash
```

Installs to `~/.local/bin` via [facile](https://github.com/FacileStudio/facile), the suite installer. Pass `--bin-dir <dir>` to change that, `--source` to build from source.

Already have `facile`:

```sh
facile install sonde
```

## Quick start

```sh
sonde login https://sonde.example.com
sonde monitors add --name site --target https://example.com
sonde status
```

Cron heartbeat for a push monitor:

```
* * * * * sonde push <token> --instance https://sonde.example.com
```

The REST endpoints behind `monitors`, `status` and `incidents` are part of Sonde's plan (`Sonde/docs/PLAN.md`, Track G); until that API ships they return errors. `push` works today against the specified heartbeat endpoint.

## Configuration

| Variable | Purpose |
|---|---|
| `SONDE_SERVER_URL` | Instance URL override |
| `SONDE_INSTANCE` | Accepted alias of the above |
| `SONDE_TOKEN` | Credential override, for CI |

Config file: `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`.

## Structure

```
main.go            entry point
cmd/               cobra commands
internal/api/      Sonde REST client
internal/config/   credential storage and resolution
internal/ui/       output vocabulary per CLI-STANDARD §7
docs/              usage and development docs
install.sh         facile shim
```

## Documentation

- [docs/usage.md](docs/usage.md) — full command reference
- [docs/development.md](docs/development.md) — building locally

Part of [Facile Studio](https://github.com/FacileStudio).
