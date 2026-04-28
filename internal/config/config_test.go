package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvOverridesYaml(t *testing.T) {
	yamlContent := `
server:
  api_port: 8080
  metrics_port: 9090
global:
  max_stale_duration: 5m
providers:
  codex:
    name: codex
    base_url: http://codex.example.com
    token: yaml-token
    refresh_interval: 10m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 30s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("UCPQA_API_PORT", "8888")
	os.Setenv("UCPQA_METRICS_PORT", "9999")
	os.Setenv("UCPQA_MAX_STALE_DURATION", "15m")
	os.Setenv("UCPQA_PROVIDER_CODEX_TOKEN", "env-token")
	os.Setenv("UCPQA_PROVIDER_CODEX_REFRESH_INTERVAL", "20m")
	defer func() {
		os.Unsetenv("UCPQA_API_PORT")
		os.Unsetenv("UCPQA_METRICS_PORT")
		os.Unsetenv("UCPQA_MAX_STALE_DURATION")
		os.Unsetenv("UCPQA_PROVIDER_CODEX_TOKEN")
		os.Unsetenv("UCPQA_PROVIDER_CODEX_REFRESH_INTERVAL")
	}()

	cfg, err := Load("", configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.APIPort != 8888 {
		t.Errorf("APIPort: got %d, want 8888", cfg.Server.APIPort)
	}
	if cfg.Server.MetricsPort != 9999 {
		t.Errorf("MetricsPort: got %d, want 9999", cfg.Server.MetricsPort)
	}
	if cfg.Global.MaxStaleDuration != 15*time.Minute {
		t.Errorf("MaxStaleDuration: got %v, want 15m", cfg.Global.MaxStaleDuration)
	}
	if cfg.Providers["codex"].Token != "env-token" {
		t.Errorf("Provider token: got %q, want env-token", cfg.Providers["codex"].Token)
	}
	if cfg.Providers["codex"].RefreshInterval != 20*time.Minute {
		t.Errorf("RefreshInterval: got %v, want 20m", cfg.Providers["codex"].RefreshInterval)
	}
}

func TestValidateRejectsMissingToken(t *testing.T) {
	yamlContent := `
server:
  api_port: 8080
  metrics_port: 9090
global:
  max_stale_duration: 5m
providers:
  codex:
    name: codex
    base_url: http://codex.example.com
    token: ""
    refresh_interval: 10m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 30s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("UCPQA_CONFIG_PATH", configPath)
	defer os.Unsetenv("UCPQA_CONFIG_PATH")

	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Load failed unexpectedly: %v", err)
	}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate: expected error for missing token, got nil")
	}
	if err.Error() != `provider "codex" token is required` {
		t.Errorf("Validate error: got %q, want %q", err.Error(), `provider "codex" token is required`)
	}
}