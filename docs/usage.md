# Usage

All commands accept a global `--instance <url>` flag and `--no-color`.

## login

```sh
sonde login [url] [--token <token> | --token-stdin] [--no-browser]
```

Default is the browser SSO flow against the instance's identity provider (porte): the CLI opens `/auth/oidc?flow=cli` with a loopback callback port and a state nonce, exchanges the returned one-time code at `/auth/oidc/exchange`, and stores the bearer token.

On a headless machine (`--no-browser`, no TTY), or when discovery at `/auth/config` reports no OIDC, the RFC 8628 device authorization flow is used instead.

`logout` clears the stored token. It is not an error to run it while logged out.

## push

```sh
sonde push <token> [--instance <url>]
```

Sends `GET {instance}/api/push/{token}`. Unauthenticated by design: the token is the monitor's credential. Intended for cron.

## monitors

```sh
sonde monitors list
sonde monitors add --name <name> --target <url-or-host> [--type http|tcp|push] [--interval 60]
sonde monitors remove <id>
```

## status

```sh
sonde status
```

One line per monitor: state, name, target.

## incidents

```sh
sonde incidents
```

Incident history with open/resolved markers.

## Credentials

Precedence: flag > environment > config file.

- `SONDE_TOKEN` overrides the stored credential (for CI).
- `SONDE_SERVER_URL`, or the accepted alias `SONDE_INSTANCE`, overrides the instance.
- Config lives at `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`, created `0600` in a `0700` directory.
