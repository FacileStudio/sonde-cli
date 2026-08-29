# sonde-cli — Configuration

The config file, the environment, and which one wins.

## The file

```
${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml
```

Two keys, written by `login` and cleared by `logout`:

```yaml
url: https://sonde.facile.studio
token: <session token>
```

The file is `0600` and its directory `0700`. Both are tightened on read, not only on create,
so a permissive mode set by hand or by a restore is corrected the next time the CLI runs
rather than silently tolerated.

Nothing else is stored. There is no cache, no state directory and no log file.

## Environment

| Variable | What it sets |
|---|---|
| `SONDE_SERVER_URL` | The instance URL. `SONDE_URL` is an accepted alias |
| `SONDE_TOKEN` | The session token, for CI and headless use |
| `SONDE_OIDC_ISSUER` | Overrides the identity provider the device grant talks to |
| `NO_COLOR` | Disables colour, as does a non-TTY stdout and `--no-color` |

## Precedence

Highest first, resolved once in `internal/config`:

```
instance:    --url  >  SONDE_SERVER_URL  >  SONDE_URL  >  config.yml  >  nothing
credential:            SONDE_TOKEN                     >  config.yml  >  nothing
```

**There is no `--token` flag, deliberately.** A credential on the command line lands in the
shell history of every machine it is typed on and in the process table of every machine it
runs on. `SONDE_TOKEN` covers the same cases without either.

`logout` warns when `SONDE_TOKEN` is set: it clears the stored session, but the environment
still outranks the file, so the shell would keep working and look like the logout failed.

## No default instance

The CLI ships with no fallback URL. An uptime monitor that quietly talks to somebody else's
instance is worse than one that refuses to run, so an unconfigured `sonde status` says so
instead of guessing.

## What needs a credential

Everything except two commands:

- `sonde status <status-page-slug>` reads a public status page, which is unauthenticated by
  design — that is what a status page is for.
- `sonde push <token>` authenticates with the monitor's own token, which is what makes it
  usable from a cron line on a machine that has never logged in.
