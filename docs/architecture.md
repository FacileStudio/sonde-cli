# sonde-cli — Architecture

How the client talks to an instance, what it stores, and the decisions that are not obvious
from the code.

## Shape

```
main.go            three lines: hands off to cmd
cmd/               one file per command, plus root.go
internal/
  api/             the HTTP surface
  config/          the instance URL and the session token
  devicegrant/     RFC 8628 device flow against the identity provider
  loopback/        the browser redirect flow, for instances without a device grant
  ui/              CLI-STANDARD §7 output vocabulary
```

Dependencies are cobra, `fatih/color`, `golang.org/x/term` and `gopkg.in/yaml.v3`. Everything
else is the standard library. A client for one API does not need an HTTP framework.

## Authentication

Sonde's API runs on the suite's `porte` kit, and the CLI asks the instance what it supports
before deciding how to log in. `GET /api/auth/config` returns `oidc_enabled` and `sso_only`,
which picks one of three paths:

1. **Device grant** — RFC 8628 against `sso.facile.studio` as the public client `facile-cli`,
   traded for a Sonde session at `POST /api/auth/oidc/device/exchange`. Preferred, because it
   works over SSH where no browser can open.
2. **Loopback** — the browser redirect flow through `/api/auth/oidc`, with a `cli_state` nonce
   minted locally and verified on the way back.
3. **Password** — only where the instance has no identity provider at all.

Both browser paths ask their questions before printing a code, so nothing is on screen that
the user has not already agreed to.

The token travels as `Authorization: Bearer …`. The instance also sets a session cookie on the
same response, and the CLI ignores it on purpose: porte reads the cookie *before* the
Authorization header and refuses a cookie-authenticated **mutating** request that carries no
`X-Facile-CSRF` header. A cookie client that forgot the header sees every read succeed and
every write 403, which reads as "the tool works, saving is broken". Bearer is exempt from that
rule by construction, because nothing attaches a bearer header on the caller's behalf.

The token is a bearer credential in a file, so the file is `0600` and its directory `0700`,
tightened on every read rather than only on creation. There is no keychain integration: on Go
that means either cgo or a dependency, and the file is what every other Go tool on the machine
already uses.

## What the CLI reads, and what it does not

`status`, `incidents` and `monitors list` are reads; `monitors add`, `monitors remove` and
`push` are writes. Editing a monitor, managing webhooks, managing status pages and the config
export are deliberately absent — they are dashboard work, and a half-implemented editor is
worse than none.

`push` is the exception to everything above: it authenticates with the monitor's own token, in
the URL, and needs no session. That is what makes it usable from a cron line on a machine that
has never run `sonde login`.

## Two facts the output layer encodes

**An absent uptime percentage is not 100%.** Every uptime field the API returns is nullable,
because a window with no checks in it was never measured. The CLI prints `—` and never a
number it did not receive. Rendering `null` as zero, or as a perfect score, are both lies and
the second one is the dangerous direction.

**`paused` is not `down`.** A monitor nobody is probing has not failed. The CLI reports the
state; it cannot currently set or clear it, which is a gap and not a design.

## Exit codes

`0` success, `1` failure, `2` usage, `130` SIGINT — CLI-STANDARD §7.3. Cobra validates
arguments before the hooks run and flags after, so `root.go` carries a `commandStarted` flag
to tell a usage error from a runtime one at the point where the distinction is still knowable.
