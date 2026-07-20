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
  cors_enabled: false
  cors_allowed_origins:
    - https://yaml.example.com
  cors_allowed_methods:
    - GET
  cors_allowed_headers:
    - Content-Type
global:
  max_stale_duration: 5m
providers:
  kimi:
    name: kimi
    base_url: http://kimi.example.com
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
	os.Setenv("UCPQA_PROVIDER_KIMI_TOKEN", "env-token")
	os.Setenv("UCPQA_PROVIDER_KIMI_REFRESH_INTERVAL", "20m")
	os.Setenv("UCPQA_CORS_ENABLED", "true")
	os.Setenv("UCPQA_CORS_ALLOWED_ORIGINS", "https://frontend.example.com,http://localhost:5173")
	os.Setenv("UCPQA_CORS_ALLOWED_METHODS", "GET,OPTIONS")
	os.Setenv("UCPQA_CORS_ALLOWED_HEADERS", "Content-Type,Authorization")
	defer func() {
		os.Unsetenv("UCPQA_API_PORT")
		os.Unsetenv("UCPQA_METRICS_PORT")
		os.Unsetenv("UCPQA_MAX_STALE_DURATION")
		os.Unsetenv("UCPQA_PROVIDER_KIMI_TOKEN")
		os.Unsetenv("UCPQA_PROVIDER_KIMI_REFRESH_INTERVAL")
		os.Unsetenv("UCPQA_CORS_ENABLED")
		os.Unsetenv("UCPQA_CORS_ALLOWED_ORIGINS")
		os.Unsetenv("UCPQA_CORS_ALLOWED_METHODS")
		os.Unsetenv("UCPQA_CORS_ALLOWED_HEADERS")
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
	if cfg.Providers["kimi"].Token != "env-token" {
		t.Errorf("Provider token: got %q, want env-token", cfg.Providers["kimi"].Token)
	}
	if cfg.Providers["kimi"].RefreshInterval != 20*time.Minute {
		t.Errorf("RefreshInterval: got %v, want 20m", cfg.Providers["kimi"].RefreshInterval)
	}
	if !cfg.Server.CorsEnabled {
		t.Errorf("CorsEnabled: got %v, want true", cfg.Server.CorsEnabled)
	}
	if len(cfg.Server.CorsAllowedOrigins) != 2 || cfg.Server.CorsAllowedOrigins[0] != "https://frontend.example.com" || cfg.Server.CorsAllowedOrigins[1] != "http://localhost:5173" {
		t.Errorf("CorsAllowedOrigins: got %v, want [https://frontend.example.com http://localhost:5173]", cfg.Server.CorsAllowedOrigins)
	}
	if len(cfg.Server.CorsAllowedMethods) != 2 || cfg.Server.CorsAllowedMethods[0] != "GET" || cfg.Server.CorsAllowedMethods[1] != "OPTIONS" {
		t.Errorf("CorsAllowedMethods: got %v, want [GET OPTIONS]", cfg.Server.CorsAllowedMethods)
	}
	if len(cfg.Server.CorsAllowedHeaders) != 2 || cfg.Server.CorsAllowedHeaders[0] != "Content-Type" || cfg.Server.CorsAllowedHeaders[1] != "Authorization" {
		t.Errorf("CorsAllowedHeaders: got %v, want [Content-Type Authorization]", cfg.Server.CorsAllowedHeaders)
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
  kimi:
    name: kimi
    base_url: http://kimi.example.com
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
	if err.Error() != `provider "kimi" token is required` {
		t.Errorf("Validate error: got %q, want %q", err.Error(), `provider "kimi" token is required`)
	}
}
