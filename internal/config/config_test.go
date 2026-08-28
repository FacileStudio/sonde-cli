package config

import (
	"os"
	"testing"
)

// TestSaveWritesOwnerOnly pins the rule the CLI standard puts a credential file
// under: 0600 on the file, 0700 on the directory, set at creation.
func TestSaveWritesOwnerOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := Save(Config{URL: "https://sonde.example.test", Token: "secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	file, err := os.Stat(Path())
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := file.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %#o, want 0600", perm)
	}
	dir, err := os.Stat(Dir())
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dir.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %#o, want 0700", perm)
	}
}

// TestLoadTightensLoosePermissions covers the installs that predate the rule. A
// tool that only writes correctly leaves every existing token exposed.
func TestLoadTightensLoosePermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := Save(Config{URL: "https://sonde.example.test", Token: "secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(Path(), 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Token != "secret" {
		t.Errorf("token = %q, want secret", loaded.Token)
	}
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode after Load = %#o, want 0600", perm)
	}
}

// TestLoadTightensTheDirectoryToo pins the directory half of the rule.
// MkdirAll's mode applies only when it creates the directory, so a
// ~/.config/sonde that already existed 0755 stays 0755 while the file inside it
// is written 0600.
func TestLoadTightensTheDirectoryToo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := Save(Config{URL: "https://sonde.example.test", Token: "secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(Dir(), 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	info, err := os.Stat(Dir())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode after Load = %#o, want 0700", perm)
	}
}

// TestURLEnvAliasLosesToTheCanonicalName pins the precedence between the two
// accepted spellings.
func TestURLEnvAliasLosesToTheCanonicalName(t *testing.T) {
	t.Setenv(URLEnv, "canonical.example.test")
	t.Setenv(URLEnvAlias, "alias.example.test")
	if got := ResolveURL("", ""); got != "https://canonical.example.test" {
		t.Errorf("with both set = %q, want the canonical name", got)
	}

	t.Setenv(URLEnv, "")
	if got := ResolveURL("", ""); got != "https://alias.example.test" {
		t.Errorf("with only the alias set = %q, want the alias", got)
	}
}

// TestClearKeepsTheInstanceURL pins that logging out does not also make the
// user retype where their instance is.
func TestClearKeepsTheInstanceURL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := Save(Config{URL: "https://sonde.example.test", Token: "secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != (Config{URL: "https://sonde.example.test"}) {
		t.Fatalf("after Clear = %+v, want the URL alone", loaded)
	}
}

// TestResolutionFollowsTheStandardLadder pins flag > environment > file.
func TestResolutionFollowsTheStandardLadder(t *testing.T) {
	t.Setenv(URLEnv, "env.example.test")
	t.Setenv(TokenEnv, "from-env")

	if got := ResolveURL("https://stored.example.test", "flag.example.test"); got != "https://flag.example.test" {
		t.Errorf("with a flag = %q, want the flag", got)
	}
	if got := ResolveURL("https://stored.example.test", ""); got != "https://env.example.test" {
		t.Errorf("with no flag = %q, want the environment", got)
	}
	if got := ResolveToken("from-file"); got != "from-env" {
		t.Errorf("token = %q, want the environment", got)
	}

	t.Setenv(URLEnv, "")
	t.Setenv(TokenEnv, "")
	if got := ResolveURL("https://stored.example.test", ""); got != "https://stored.example.test" {
		t.Errorf("with nothing set = %q, want the stored URL", got)
	}
	if got := ResolveToken("from-file"); got != "from-file" {
		t.Errorf("token = %q, want the stored token", got)
	}
}
