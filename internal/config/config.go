package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	URL   string `yaml:"url,omitempty"`
	Token string `yaml:"token,omitempty"`
}

const (
	TokenEnv  = "SONDE_TOKEN"
	URLEnv    = "SONDE_SERVER_URL"
	URLEnvAlt = "SONDE_INSTANCE"
)

func ConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sonde", "config.yml")
}

func Load() (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", ConfigPath(), err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ConfigPath(), err)
	}
	tighten()
	return cfg, nil
}

func Save(cfg *Config) error {
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	fmt.Printf("Config saved to %s\n", path)
	return nil
}

func (c *Config) ServerURL() string {
	for _, key := range []string{URLEnv, URLEnvAlt} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	return strings.TrimRight(c.URL, "/")
}

func (c *Config) AuthToken() string {
	if value := strings.TrimSpace(os.Getenv(TokenEnv)); value != "" {
		return value
	}
	return c.Token
}

func tighten() {
	path := ConfigPath()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0077 == 0 {
		return
	}
	os.Chmod(path, 0600)
}
