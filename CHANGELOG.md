# Changelog

All notable changes to this project are documented here. The format is
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While on
`0.x`, a breaking change bumps the minor.

## [Unreleased]

Nothing yet.

## [0.2.0] — 2026-08-29

### Added

- `integrations/SKILL.md`, so `facile install sonde` registers the agent skill
  and Claude and Codex can drive the CLI. Sonde was the one suite tool an agent
  could not reach. Paired with `skill: sonde` in facile's catalog.
- `docs/architecture.md` and `docs/configuration.md`, the two pages the docs
  standard makes mandatory.

### Changed

- `docs/README.md` and the README's documentation block are tables, and the
  README carries the suite footer.
- The install section documents `--no-skill`, which now has something to skip.

## [0.1.0] — 2026-08-28

### Added

- First release. `push`, `monitors list/add/remove`, `status` and `incidents`
  against a Sonde instance's REST API, plus `login` and `logout`.
- Sign-in through the identity provider's RFC 8628 device grant, traded for a
  Sonde session at porte's device exchange, with porte's loopback SSO flow as
  the same-machine path and a password login for an instance with no identity
  provider. The loopback listener mints a `cli_state` nonce and refuses a
  callback that does not echo it.
- `--json` on every command that carries data, forcing colour off, per
  CLI-STANDARD §8. `status` emits the table as data, so a script gets the
  verdict and the uptime ratio without reassembling them from two endpoints.
- Exit codes per CLI-STANDARD §7.3: 0 success, 1 error, 2 usage error, 130 on
  Ctrl-C. The usage code comes from a flag set once the command body starts,
  because cobra validates args before its hooks run and flags after them.
- GoReleaser config, a tag-triggered release workflow, and a Homebrew cask on
  `FacileStudio/homebrew-tap`. `facile install sonde` and the `install.sh` shim
  cover the rest.
- Tighten-on-read for both the credential file and its directory. `MkdirAll`'s
  mode applies only when it creates the directory, so a `~/.config/sonde` that
  already existed `0755` kept that mode while the token inside it was written
  `0600`.
- `SONDE_OIDC_ISSUER` must name an https provider, or a loopback one. Everything
  the device grant trusts comes out of the discovery document, so plaintext lets
  anyone on the path choose the page the human is told to open. `OpenBrowser`
  refuses any URL that is not http or https for the same reason: `open` and
  `xdg-open` dispatch on scheme, and the verification address is the provider's
  to choose.

### Changed

- `--url` is the instance flag, which is the name every other Go CLI in the
  suite uses. `SONDE_SERVER_URL` stays the canonical environment variable and
  `SONDE_URL` is an accepted alias.
- A device login that could not reach the provider is reported as that, and
  fails, rather than being collapsed into "the provider does not offer the
  device grant" and falling back to a browser flow the headless machine that
  needs the device grant cannot run. An instance with no device exchange is
  named as the instance, not as the provider.

[Unreleased]: https://github.com/FacileStudio/sonde-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/FacileStudio/sonde-cli/releases/tag/v0.1.0
