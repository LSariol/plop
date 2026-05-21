package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type TagRule struct {
	Tag    string `toml:"tag"`
	Folder string `toml:"folder"`
}

type Tags struct {
	DefaultFolder string    `toml:"default_folder"`
	Rules         []TagRule `toml:"rules"`
}

type Config struct {
	ServerURL   string `toml:"server_url"`
	MachineName string `toml:"machine_name"`
	PairingCode string `toml:"pairing_code,omitempty"`
	Tags        Tags   `toml:"tags"`
}

const defaultTemplate = `server_url   = "https://your-server-here.com"
machine_name = "My PC"

# Step 1: Log in at your server and click "Pair Desktop" to get a code
# Step 2: Add it below and restart:
# pairing_code = "XXXXXXXX"

[tags]
default_folder = "C:/Users/You/Desktop/Plop"

# [[tags.rules]]
# tag    = "work"
# folder = "C:/Users/You/Documents/Work"
`

func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if writeErr := os.WriteFile(path, []byte(defaultTemplate), 0644); writeErr != nil {
			return Config{}, fmt.Errorf("config not found and could not write default to %s: %w", path, writeErr)
		}
		return Config{}, fmt.Errorf("config not found — default written to %s\nEdit it and restart", path)
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.ServerURL == "" {
		return Config{}, fmt.Errorf("server_url is required in %s", path)
	}
	if cfg.MachineName == "" {
		return Config{}, fmt.Errorf("machine_name is required in %s", path)
	}
	if cfg.Tags.DefaultFolder == "" {
		return Config{}, fmt.Errorf("tags.default_folder is required in %s", path)
	}

	return cfg, nil
}
