package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DirName      = "tinytui"
	ConfigName   = "config.json"
	EnvAPIKey    = "TINYPNG_API_KEY"
	PermFile     = 0600
	PermDir      = 0700
)

type MascotMode string

const (
	MascotOff  MascotMode = "off"
	MascotOn   MascotMode = "on"
	MascotAuto MascotMode = "auto"
)

type Config struct {
	APIKey       string     `json:"api_key"`
	OutputMode   string     `json:"output_mode"` // "replace" or "directory"
	OutputDir    string     `json:"output_dir,omitempty"`
	Suffix       string     `json:"suffix"`
	Metadata     bool       `json:"metadata"`
	Mascot       MascotMode `json:"mascot"`
	configPath   string
}

func DefaultConfig() *Config {
	return &Config{
		OutputMode: "replace",
		Suffix:     ".tiny",
		Metadata:   false,
		Mascot:     MascotAuto,
	}
}

// Load reads the configuration from the standard config location.
// It also checks the TINYPNG_API_KEY environment variable.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(configDir, DirName, ConfigName)
	cfg.configPath = path

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// If not exists, check env var first
		if envKey := os.Getenv(EnvAPIKey); envKey != "" {
			cfg.APIKey = envKey
		}
		// Return default with potentially env key set.
		// We don't save yet.
		return cfg, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Environment variable overrides config file
	if envKey := os.Getenv(EnvAPIKey); envKey != "" {
		cfg.APIKey = envKey
	}

	return cfg, nil
}

// Save writes the configuration to the file with strict permissions.
func (c *Config) Save() error {
	if c.configPath == "" {
		// Re-derive path if missing (shouldn't happen if loaded via Load)
		configDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		c.configPath = filepath.Join(configDir, DirName, ConfigName)
	}

	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, PermDir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, PermFile)
}

// ShouldShowMascot determines if mascot should be shown based on logic and terminal width
func (c *Config) ShouldShowMascot(termWidth int) bool {
	switch c.Mascot {
	case MascotOff:
		return false
	case MascotOn:
		return true
	case MascotAuto:
		return termWidth >= 100
	default:
		return termWidth >= 100
	}
}

// IsConfigured returns true if the API key is set.
func (c *Config) IsConfigured() bool {
	return c.APIKey != ""
}
