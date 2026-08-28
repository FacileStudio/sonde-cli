// Package config stores which Sonde instance the CLI talks to and the session
// it holds for it. The file is the lowest rung of the precedence ladder the CLI
// standard requires: flag, then environment, then this, then nothing.
package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TokenEnv overrides the stored credential. A CI job cannot run an interactive
// login and must not commit a config file, so this is the only credential
// channel it has.
const TokenEnv = "SONDE_TOKEN"

// URLEnv overrides the stored instance URL. It is the canonical spelling the
// CLI standard gives it, and it wins when both spellings are set.
const URLEnv = "SONDE_SERVER_URL"

// URLEnvAlias is the shorter spelling the rest of the suite accepts alongside
// the canonical one, so a profile that already exports it keeps working.
const URLEnvAlias = "SONDE_URL"

// Config is the whole stored state. There is no default instance URL on
// purpose: Sonde is self-hosted and guessing an address for it would send a
// first login somewhere that is not the user's.
type Config struct {
	URL   string `yaml:"url,omitempty"`
	Token string `yaml:"token,omitempty"`
}

// Dir is the configuration directory, honouring XDG_CONFIG_HOME and falling
// back to a relative dotdir when the home directory cannot be resolved.
func Dir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "sonde")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sonde"
	}
	return filepath.Join(home, ".config", "sonde")
}

// Path is the configuration file.
func Path() string { return filepath.Join(Dir(), "config.yml") }

// Load reads the configuration, returning an empty one when none exists yet so
// a first run is not an error.
//
// It also tightens the file to 0600 when it is found with any group or other
// bit set. Installs predate the permission rule, and a tool that only writes
// correctly leaves every existing token exposed.
func Load() (Config, error) {
	var cfg Config

	if err := secureDir(); err != nil {
		return cfg, err
	}

	file, err := os.Open(Path())
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return cfg, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := file.Chmod(0o600); err != nil {
			return cfg, err
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.URL = NormalizeURL(cfg.URL)
	return cfg, nil
}

// secureDir tightens the configuration directory to 0700 when it exists with
// any group or other bit set, and is a no-op when it does not exist yet.
//
// It is the directory half of the tighten-on-read rule the file already
// follows. MkdirAll's mode applies only when it creates the directory, so a
// ~/.config/sonde that predates this CLI, or that another tool created 0755,
// keeps that mode forever while the token inside it is diligently 0600. A
// world-readable directory does not expose the file's bytes, but it does list
// the file and leaves the mode one careless umask away from mattering.
func secureDir() error {
	info, err := os.Stat(Dir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	return os.Chmod(Dir(), 0o700)
}

// Save writes the configuration owner-only. The file holds a bearer token, so
// the mode is set at creation rather than chmod'd afterwards: writing first and
// fixing the mode second leaves a window in which the token is world-readable,
// and on a shared machine that window is the whole attack.
func Save(cfg Config) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	if err := secureDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(Path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// Clear removes the stored session but keeps the instance URL, so logging out
// does not also make the user retype where their instance is.
//
// A file that cannot be parsed still loses its token. Logout is what somebody
// reaches for on a borrowed machine, and refusing because the YAML is malformed
// would leave a working credential exactly where they tried to remove it.
func Clear() error {
	cfg, err := Load()
	if err != nil {
		cfg = Config{}
	}
	cfg.Token = ""
	return Save(cfg)
}

// NormalizeURL trims a trailing slash and supplies a scheme, so
// `sonde login sonde.example.com` works as typed. An empty input stays empty
// rather than becoming a bare scheme.
func NormalizeURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return "https://" + trimmed
	}
	return trimmed
}

// ResolveURL applies the precedence the CLI standard requires, highest first:
// the flag, SONDE_SERVER_URL, then what login stored.
func ResolveURL(stored, flag string) string {
	if flag != "" {
		return NormalizeURL(flag)
	}
	if fromEnv := urlFromEnv(); fromEnv != "" {
		return NormalizeURL(fromEnv)
	}
	return NormalizeURL(stored)
}

// urlFromEnv reads the instance URL, preferring the canonical
// SONDE_SERVER_URL over the SONDE_URL alias.
func urlFromEnv() string {
	if value := strings.TrimSpace(os.Getenv(URLEnv)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(URLEnvAlias))
}

// ResolveToken applies the same ladder to the credential. There is no flag: a
// token on a command line lands in the shell history and in the process table.
func ResolveToken(stored string) string {
	if fromEnv := strings.TrimSpace(os.Getenv(TokenEnv)); fromEnv != "" {
		return fromEnv
	}
	return stored
}
