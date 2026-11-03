# sonde-cli

Terminal client for Sonde, the Facile suite's self-hosted uptime monitor. It
pushes heartbeats from cron jobs and reads monitors, status and incidents from
the shell.

## What it does

- `sonde push <token>`: cron-friendly heartbeat for a push monitor, no login
- Signs in through the suite identity provider's device grant, so a server whose
  browser is on another machine can still log in, with the loopback browser flow
  as the fallback
- `sonde monitors list/add/remove`, `sonde status`, `sonde incidents`, `sonde keys list/create/revoke`
- `--json` on every command that carries data
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

`facile install` also registers the agent skill in `integrations/SKILL.md`, so Claude and
Codex can drive the CLI. Pass `--no-skill` to install the binary alone.

## Quick start

```sh
sonde login
sonde monitors add --slug site --name Site --target https://example.com
sonde status
```

A push monitor and its cron line:

```sh
sonde monitors add --slug backups --name "Nightly backups" --type push --interval 86400
# prints the push token
```

```
0 3 * * * sonde push <token> --url https://sonde.example.com
```

## Signing in

`sonde login` with no URL signs in to https://sonde.facile.studio. It opens
your browser, your existing Facile session completes the login with no prompt,
and the CLI stores the session. Nothing to copy or paste.

- **Browser (default)**: a one-time code comes back on `127.0.0.1` and is
  exchanged at `/api/auth/oidc/exchange`.
- **Device grant**, on a machine with no browser (SSH, CI): it prints a short
  code to approve from a logged-in session on any device.
- **Password**, on an instance with no identity provider configured.
- **Token**: `sonde login --token <token>` or `--token-stdin`.

Pass a URL to sign in to a self-hosted instance.

## Configuration

| Variable | Purpose |
|---|---|
| `SONDE_SERVER_URL` | Instance URL, overriding the stored one. `--url` beats both. `SONDE_URL` is an accepted alias |
| `SONDE_TOKEN` | Credential, for CI |
| `SONDE_OIDC_ISSUER` | Identity provider for the device grant. Must be https, or loopback |

Config file: `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`. With nothing
configured the CLI talks to https://sonde.facile.studio.

## Structure

```
main.go                  entry point
cmd/                     cobra commands
internal/api/            Sonde REST client
internal/config/         credential storage and resolution
internal/devicegrant/    RFC 8628 against sso.facile.studio: constants and
                         transport, discovery.go, poll.go
internal/loopback/       the same-machine browser flow
internal/ui/             output vocabulary per CLI-STANDARD §7
docs/                    usage and development docs
install.sh               facile shim
```

## Documentation

| Doc | What's in it |
|---|---|
| [Architecture](docs/architecture.md) | How the client talks to an instance, and why it stores what it does |
| [Configuration](docs/configuration.md) | The config file, the environment, and precedence |
| [Development](docs/development.md) | Local setup, the quality gate, releasing |
| [Usage](docs/usage.md) | Every command and flag |
| [Changelog](CHANGELOG.md) | What shipped, per release |

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
