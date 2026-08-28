# Development

Requires Go 1.26, via [mise](https://mise.jdx.dev).

```sh
mise run build    # go build -o bin/sonde .
mise run check    # sh scripts/check.sh — gofmt, vet, test
mise run format   # rewrite Go sources in place
filet check .     # the suite's code-quality rules, config in filet.yml
```

`scripts/check.sh` is the gate and lefthook runs it on pre-push. It depends on
nothing but a `go` on PATH.

## Layout

```
main.go                    entry point
cmd/                       cobra commands
internal/api/              Sonde REST client (client.go, auth.go, monitors.go)
internal/config/           credential storage and the precedence ladder
internal/devicegrant/      RFC 8628 against sso.facile.studio
internal/loopback/         the same-machine browser flow
internal/ui/               output vocabulary per CLI-STANDARD §7
```

## Where the contracts come from

Nothing here is invented. When one of these moves, this CLI moves with it:

- **Paths and payloads** are transcribed from `Sonde/apps/api`:
  `internal/httpapi/router.go` for monitors, incidents and the public status
  page, `internal/checker/push.go` for the heartbeat, and the porte kits
  `main.go` mounts under `/api` for everything under `/api/auth`.
- **The device exchange** is `porte.RouteDeviceExchange`, whose path and wire
  shape are frozen by the shipped caller (`facile@5483f18`). It takes
  `{"access_token": "..."}` and answers `{"user_id": "...", "token": "..."}`.
  A 404 is the signal that an instance has not shipped it.
- **The device grant** runs against Registre at `sso.facile.studio` as the
  public client `facile-cli`, which its `seed.yaml` registers with
  `grant_types: [device_code, refresh_token]`. `facile`'s
  `internal/authflow/devicegrant.go` is the reference implementation; this is
  the same protocol, not a second dialect of it.

## Releases

Tag `vX.Y.Z` and push it. `.github/workflows/release.yml` runs GoReleaser,
which publishes the archives and checksums `facile install sonde` downloads and
updates the Homebrew cask in `FacileStudio/homebrew-tap`. The cask step needs
`HOMEBREW_TAP_GITHUB_TOKEN` in the repository secrets.
