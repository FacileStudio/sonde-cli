# Usage

Every command accepts `--url <url>`, `--json` and `--no-color`.

`--json` prints one document to stdout and nothing else, and forces colour off.
Every command that carries data honours it: `monitors list`, `monitors add`,
`monitors remove`, `status`, `incidents`, `keys list`, `keys create`, and `keys revoke`.

Exit codes are `0` success, `1` failure, `2` usage error, `130` on Ctrl-C.

## login

```sh
sonde login [url] [--token <token> | --token-stdin] [--no-browser]
```

The instance is asked what it accepts at `GET /api/auth/config` before anything
is typed. Where it offers single sign-on there are two browser paths, and the
CLI picks between them before it prints anything:

1. **Device grant.** It probes `POST /api/auth/oidc/device/exchange` with an
   empty body. A 404 means this instance has not shipped the endpoint; anything
   else, including the 400 an empty body earns, means it has. It then reads
   `https://sso.facile.studio/.well-known/openid-configuration` and checks that
   `grant_types_supported` lists
   `urn:ietf:params:oauth:grant-type:device_code` — an advertised endpoint is
   not an implemented grant. With both answers yes it runs RFC 8628 as the
   public client `facile-cli`, prints a short code to enter at the provider,
   polls the token endpoint (backing off cumulatively on `slow_down`), and
   trades the resulting access token for a Sonde session at the exchange.
2. **Loopback.** Otherwise it binds `127.0.0.1:0`, opens
   `/api/auth/oidc?flow=cli&port=<port>&cli_state=<nonce>`, accepts exactly the
   callback that echoes the nonce back, and trades the one-time code at
   `/api/auth/oidc/exchange`.

Both questions are asked before any code is printed. Discovering afterwards that
the instance cannot trade a token would make the human read a code off one
screen, type it into another, wait for the poll — and land on the loopback login
anyway, which is the flow that cannot work when the browser is on another
machine.

An instance with no identity provider prompts for an address and a password at
`POST /api/auth/login`. Under `SSO_ONLY` porte does not register that route at
all, so it answers 404 and the CLI says so rather than reporting a typo.

## logout

```sh
sonde logout
```

Revokes the session at `POST /api/auth/logout`, then clears it locally. The
local credential is cleared even when the revocation fails, and the instance URL
is kept. Running it while signed out is not an error.

## push

```sh
sonde push <token> [--url <url>]
```

Sends `POST {instance}/api/push/{token}`. Unauthenticated by design: the token
in the path is the monitor's whole credential, and it is what `monitors add`
prints when it creates a push monitor. An unknown token is a 404, so a typo
fails loudly instead of reporting a dead job alive.

The cron line:

```
* * * * * sonde push <token> --url https://sonde.example.com
```

## monitors

```sh
sonde monitors list
sonde monitors add --slug <slug> --name <name> [--type http|tcp|push] [--target <url-or-host:port>]
                   [--interval 60] [--timeout 10] [--expect-status 200] [--expect-keyword <text>]
sonde monitors remove <id|slug>
```

The slug is required and is validated server-side as lowercase letters, digits
and dashes. An `http` monitor takes a URL, a `tcp` monitor takes `host:port`,
and a `push` monitor takes no target at all — its push token is the endpoint,
and `add` prints it once. The interval must be at least 20 seconds and the
timeout shorter than it.

`remove` accepts a slug and resolves it, because the API route takes an id.

## status

```sh
sonde status [status-page-slug] [--window 24h|7d|30d|90d]
```

With a slug it reads `GET /api/public/status/{slug}`, which needs no credential
and carries the instance's own up/down verdict from the newest check.

Without one it reads your monitors, their uptime over the window, and the
incidents still open against them. Sonde has no authenticated "state of
everything" route, so `down` comes from an unresolved incident, `paused` from a
disabled monitor, and a monitor with no checks yet reads `unknown` rather than a
fabricated green.

## incidents

```sh
sonde incidents [--monitor <id|slug>]
```

Newest first, capped at 200 by the instance. An incident with no resolved time
is still running.

## keys

```sh
sonde keys list [--app <name>]
sonde keys create --app <name> [--public] [--origins <urls>] [--quota <N>]
sonde keys revoke <id> [--yes]
```

Manages secret and public API keys. `create` outputs the raw one-time token to
stdout (or as part of the JSON payload under `--json`). `revoke` accepts an
integer key ID.

## Credentials

Precedence: flag > environment > config file.

- `SONDE_TOKEN` overrides the stored credential, which is the only credential
  channel a CI job has.
- `SONDE_SERVER_URL` overrides the instance. `SONDE_URL` is an accepted alias,
  and the canonical spelling wins when both are set.
- `SONDE_OIDC_ISSUER` points the device grant at another identity provider. It
  must be `https`, or a loopback address: everything the grant trusts comes out
  of the provider's discovery document.
- Config lives at `${XDG_CONFIG_HOME:-~/.config}/sonde/config.yml`, created
  `0600` in a `0700` directory, and tightened on read if it is found looser.

There is no default instance URL. Sonde is self-hosted and guessing an address
would send a first login somewhere that is not yours.
