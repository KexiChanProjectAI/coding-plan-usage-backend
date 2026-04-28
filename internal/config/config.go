package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	Server   ServerConfig             `yaml:"server"`
	Global   GlobalConfig             `yaml:"global"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	APIPort     int `yaml:"api_port"`
	MetricsPort int `yaml:"metrics_port"`
}

// GlobalConfig holds application-wide settings.
type GlobalConfig struct {
	MaxStaleDuration time.Duration `yaml:"max_stale_duration"`
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	Type            string          `yaml:"type"`              // Provider type (codex, kimi, minimax, zai, zhipu)
	Name            string          `yaml:"name"`              // Human-readable alias (kept for backward compat)
	BaseURL         string          `yaml:"base_url"`
	Token           string          `yaml:"token"`
	RefreshInterval time.Duration   `yaml:"refresh_interval"`
	JitterPercent   int             `yaml:"jitter_percent"`
	BackoffInitial  time.Duration   `yaml:"backoff_initial"`
	BackoffMax      time.Duration   `yaml:"backoff_max"`
}

var validProviderTypes = map[string]bool{
	"codex":   true,
	"kimi":    true,
	"minimax": true,
	"zai":     true,
	"zhipu":   true,
}

// Validate checks that all required fields are present.
func (c *Config) Validate() error {
	for name, prov := range c.Providers {
		if prov.Token == "" {
			return fmt.Errorf("provider %q token is required", name)
		}
		// If Type is not specified, fall back to config key for backward compatibility
		if prov.Type != "" && !validProviderTypes[strings.ToLower(prov.Type)] {
			return fmt.Errorf("provider %q has unknown type %q (valid types: codex, kimi, minimax, zai, zhipu)", name, prov.Type)
		}
	}
	return nil
}

// Load reads configuration from a YAML file and applies environment overrides.
// The configPath can be set via UCPQA_CONFIG_PATH env var; otherwise uses defaultPath.
func Load(configPath string, defaultPath string) (*Config, error) {
	path := configPath
	if path == "" {
		path = os.Getenv("UCPQA_CONFIG_PATH")
	}
	if path == "" {
		path = defaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables use the prefix "UCPQA_" followed by the config path.
func applyEnvOverrides(cfg *Config) {
	// Server config
	if v := os.Getenv("UCPQA_API_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.APIPort = port
		}
	}
	if v := os.Getenv("UCPQA_METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.MetricsPort = port
		}
	}

	// Global config
	if v := os.Getenv("UCPQA_MAX_STALE_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Global.MaxStaleDuration = d
		}
	}

	// Provider configs
	for name := range cfg.Providers {
		upperName := strings.ToUpper(name)
		prov := cfg.Providers[name]

		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_BASE_URL", upperName)); v != "" {
			prov.BaseURL = v
		}
		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_TOKEN", upperName)); v != "" {
			prov.Token = v
		}
		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_REFRESH_INTERVAL", upperName)); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				prov.RefreshInterval = d
			}
		}
		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_JITTER_PERCENT", upperName)); v != "" {
			if j, err := strconv.Atoi(v); err == nil {
				prov.JitterPercent = j
			}
		}
		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_BACKOFF_INITIAL", upperName)); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				prov.BackoffInitial = d
			}
		}
		if v := os.Getenv(fmt.Sprintf("UCPQA_PROVIDER_%s_BACKOFF_MAX", upperName)); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				prov.BackoffMax = d
			}
		}
		cfg.Providers[name] = prov
	}
}