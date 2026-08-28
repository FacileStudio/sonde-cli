# sonde-cli

Terminal client for Sonde, the Facile suite's self-hosted uptime monitor. It
pushes heartbeats from cron jobs and reads monitors, status and incidents from
the shell.

## What it does

- `sonde push <token>` — cron-friendly heartbeat for a push monitor, no login
- Signs in through the suite identity provider's device grant, so a server whose
  browser is on another machine can still log in, with the loopback browser flow
  as the fallback
- `sonde monitors list/add/remove`, `sonde status`, `sonde incidents`
- Credential precedence flag > environment (`SONDE_TOKEN`, `SONDE_SERVER_URL`) >
  config file
- Config at `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`, created owner-only

## Stack

- Go 1.26, cobra for the command surface
- GoReleaser for releases, `facile` for installation

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/FacileStudio/sonde-cli/main/install.sh | bash
```

Installs to `~/.local/bin` via [facile](https://github.com/FacileStudio/facile),
the suite installer. Pass `--bin-dir <dir>` to change that, `--source` to build
from source.

Already have `facile`:

```sh
facile install sonde
```

## Quick start

```sh
sonde login https://sonde.example.com
sonde monitors add --slug site --name Site --target https://example.com
sonde status
```

A push monitor and its cron line:

```sh
sonde monitors add --slug backups --name "Nightly backups" --type push --interval 86400
# prints the push token
```

```
0 3 * * * sonde push <token> --instance https://sonde.example.com
```

## Signing in

`sonde login` asks the instance what it accepts, then picks a flow:

- **Device grant**, when the instance serves `POST
  /api/auth/oidc/device/exchange` and `sso.facile.studio` advertises
  `urn:ietf:params:oauth:grant-type:device_code`. It prints a short code to
  enter on any device, then trades the provider's token for a Sonde session.
  This is the path for a server, where the loopback flow cannot work.
- **Loopback**, otherwise: a browser on this machine, a one-time code back on
  `127.0.0.1`, exchanged at `/api/auth/oidc/exchange`.
- **Password**, on an instance with no identity provider configured.

Both questions are asked before a code is printed, so a machine that would end
up on the loopback flow never makes anybody type a code first.

## Configuration

| Variable | Purpose |
|---|---|
| `SONDE_SERVER_URL` | Instance URL |
| `SONDE_TOKEN` | Credential, for CI |

Config file: `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`. There is no
default instance URL: Sonde is self-hosted, and guessing an address would send a
first login somewhere that is not yours.

## Structure

```
main.go                  entry point
cmd/                     cobra commands
internal/api/            Sonde REST client
internal/config/         credential storage and resolution
internal/devicegrant/    RFC 8628 against sso.facile.studio
internal/loopback/       the same-machine browser flow
internal/ui/             output vocabulary per CLI-STANDARD §7
docs/                    usage and development docs
install.sh               facile shim
```

## Documentation

- [docs/usage.md](docs/usage.md) — full command reference
- [docs/development.md](docs/development.md) — building, testing, releasing

Part of [Facile Studio](https://github.com/FacileStudio).
