---
name: sonde
description: >
  Facile uptime monitoring CLI. Use when the user asks whether a site or service
  is up, wants to add or remove a monitor, read incidents or uptime, push a
  heartbeat from a cron job, manage API keys, or mentions Sonde, downtime, or a status page.
---

# sonde: Facile uptime monitoring

Binary: `sonde`
Config: `<config_dir>/sonde/config.yml` (instance URL + session token)

Sonde probes HTTP endpoints, TCP ports and push heartbeats, opens an incident
when a monitor fails three checks in a row, and serves public status pages.
This CLI reads all of that, manages API keys, and pushes heartbeats, without the dashboard.

## When to apply

Use when the user asks whether something is up or down, how much downtime
there was, what broke and when, wants a cron job to report in, or wants to manage API keys.
Triggers: "is it up", "down", "downtime", "uptime", "monitor", "incident",
"status page", "heartbeat", "sonde", "did it go down", "api key", "api keys"

## Commands

### Setup
```
sonde login [url]                 Authenticate (browser under SSO, or password)
sonde logout                      Revoke the stored session
```

### Reading
```
sonde status                      How every monitor is doing, with uptime
sonde status --window 7d          Uptime window: 24h (default), 7d, 30d, 90d
sonde status <status-page-slug>   A public status page, no session needed
sonde incidents                   Every incident, newest first
sonde incidents --monitor <id|slug>   Narrow to one monitor
sonde monitors list               The monitors themselves, with their config
```

### Writing
```
sonde monitors add --slug <s> --name <n> [--type http|tcp|push]
                   [--target <url|host:port>] [--interval 60] [--timeout 10]
                   [--expect-status 200] [--expect-keyword <text>]
sonde monitors remove <id|slug>
sonde push <token>                Report a push monitor alive
```

### API keys
```
sonde keys list [--app <name>]
sonde keys create --app <name> [--public] [--origins <urls>] [--quota <N>]
sonde keys revoke <id> [--yes]
```

## Rules
- A session is required for everything except `sonde status <slug>`, which
  reads a public status page, and `sonde push`, which authenticates with the
  monitor's own token. Run `sonde login` once, or set `SONDE_TOKEN` in CI.
- `--json` on every command carrying data, forcing colour off and leaving
  colour rules to the consumer.
- A monitor with no checks recorded reports **no** uptime percentage, not
  100%. An absent number means nothing was measured, and treating it as a
  perfect score is the one misreading that matters here.
- `paused` is a monitor nobody is probing, not a monitor that failed. The CLI
  reports it and cannot currently produce or clear it, that needs the web UI.
- **`sonde push` must be POST and is the whole heartbeat.** A monitor of type
  `push` goes down on silence past its interval, so a cron line that fails
  silently is indistinguishable from the job dying. Use
  `curl -fsS -X POST <url>` if you shell out instead: a GET answers 200 with
  HTML from the SPA catch-all and records nothing.
- `--type push` takes no `--target`; the generated token is the endpoint, and
  `monitors list --json` is where you read it back.
- `SONDE_SERVER_URL` overrides the stored instance (`SONDE_URL` is an accepted
  alias); `--url` overrides both.
- Exit: `0` success, `1` failure, `2` usage, `130` SIGINT.
- Not covered by the CLI, use the dashboard: editing a monitor, webhooks,
  managing status pages, and the config export.
