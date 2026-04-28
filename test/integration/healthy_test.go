package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quotahub/ucpqa/internal/app"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/codex"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/kimi"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/minimax"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/monitorquota"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/quotahub/ucpqa/internal/runtime/syncmanager"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"

	providertype "github.com/quotahub/ucpqa/internal/domain/provider"
)

func TestHealthyFullStackUsageSnapshot(t *testing.T) {
	// Create mock HTTP servers for each provider
	codexServer := httpmock.New()
	defer codexServer.Close()

	kimiServer := httpmock.New()
	defer kimiServer.Close()

	minimaxServer := httpmock.New()
	defer minimaxServer.Close()

	zaiServer := httpmock.New()
	defer zaiServer.Close()

	zhipuServer := httpmock.New()
	defer zhipuServer.Close()

	// Configure Codex mock response - endpoint: GET /wham/usage
	// Codex has: primary_window (5H), secondary_window (1W), additional_rate_limits (1M)
	now := time.Now()
	resetAt5H := now.Add(5 * time.Hour).Unix()
	resetAt1W := now.Add(7 * 24 * time.Hour).Unix()
	resetAt1M := now.Add(30 * 24 * time.Hour).Unix()
	codexResp := map[string]interface{}{
		"plan_type": "pro",
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         45.5,
				"limit_window_seconds": 18000, // 5H
				"reset_at":             resetAt5H,
			},
			"secondary_window": map[string]interface{}{
				"used_percent":         20.0,
				"limit_window_seconds": 604800, // 1W
				"reset_at":             resetAt1W,
			},
		},
		"additional_rate_limits": []map[string]interface{}{
			{
				"limit_name":      "monthly_limit",
				"metered_feature": "api_calls",
				"rate_limit": map[string]interface{}{
					"used_percent":         10.0,
					"limit_window_seconds": 2592000, // 1M
					"reset_at":             resetAt1M,
				},
			},
		},
	}
	err := codexServer.SetResponse("/wham/usage", 200, codexResp)
	require.NoError(t, err)

	// Configure Kimi mock response - endpoint: GET /coding/v1/usages
	// Kimi: all numeric values are strings
	kimiResetAt5H := now.Add(4 * time.Hour).Format(time.RFC3339)
	kimiResetAt1W := now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	kimiResp := map[string]interface{}{
		"usage": map[string]string{
			"limit":     "1000",
			"remaining": "600",
			"resetTime": kimiResetAt5H,
		},
		"limits": []map[string]interface{}{
			{
				"window": map[string]interface{}{
					"duration": 168, // hours
					"timeUnit": "TIME_UNIT_HOUR",
				},
				"detail": map[string]string{
					"limit":     "5000",
					"remaining": "3500",
					"resetTime": kimiResetAt1W,
				},
			},
		},
	}
	err = kimiServer.SetResponse("/coding/v1/usages", 200, kimiResp)
	require.NoError(t, err)

	// Configure MiniMax mock response - endpoint: GET /v1/api/openplatform/coding_plan/remains
	// MiniMax: current_interval_usage_count means REMAINING, base_resp.status_code must be 0
	minimaxResp := map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"start_time":                   now.Add(-1 * time.Hour).UnixMilli(),
				"end_time":                     now.Add(4 * time.Hour).UnixMilli(),
				"current_interval_total_count": 1000,
				"current_interval_usage_count": 700, // remaining = 700, used = 300
				"model_name":                   "speech-02",
				"current_weekly_total_count":   5000,
				"current_weekly_usage_count":   2000, // remaining = 3000, used = 2000
				"weekly_start_time":            now.Add(-7 * 24 * time.Hour).UnixMilli(),
				"weekly_end_time":              now.Add(7 * 24 * time.Hour).UnixMilli(),
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	}
	err = minimaxServer.SetResponse("/v1/api/openplatform/coding_plan/remains", 200, minimaxResp)
	require.NoError(t, err)

	// Configure Z.ai (MonitorQuota) mock response - endpoint: GET /api/monitor/usage/quota/limit
	// unit=3 -> 5H, unit=5 -> 1M
	zaiResp := map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "hourly",
					"unit":          3, // 5H
					"number":        1000,
					"usage":         450,
					"currentValue":  1000,
					"remaining":     550,
					"nextResetTime": now.Add(1 * time.Hour).UnixMilli(),
				},
			},
			"level": "pro",
		},
	}
	err = zaiServer.SetResponse("/api/monitor/usage/quota/limit", 200, zaiResp)
	require.NoError(t, err)

	// Configure Zhipu (MonitorQuota) mock response
	zhipuResetAt := now.Add(15 * 24 * time.Hour).UnixMilli()
	zhipuResp := map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "monthly",
					"unit":          5, // 1M
					"number":        5000,
					"usage":         1500,
					"currentValue":  5000,
					"remaining":     3500,
					"nextResetTime": zhipuResetAt,
				},
			},
			"level": "enterprise",
		},
	}
	err = zhipuServer.SetResponse("/api/monitor/usage/quota/limit", 200, zhipuResp)
	require.NoError(t, err)

	// Create config with all providers pointing to mock servers
	apiPort := 18080
	metricsPort := 18090
	maxStaleDuration := 24 * time.Hour

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     apiPort,
			MetricsPort: metricsPort,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: maxStaleDuration,
		},
		Providers: map[string]config.ProviderConfig{
			"codex": {
				Name:            "codex",
				BaseURL:         codexServer.URL(),
				Token:           "test-token-codex",
				RefreshInterval: 100 * time.Millisecond,
				JitterPercent:   0,
				BackoffInitial:  100 * time.Millisecond,
				BackoffMax:      500 * time.Millisecond,
			},
			"kimi": {
				Name:            "kimi",
				BaseURL:         kimiServer.URL(),
				Token:           "test-token-kimi",
				RefreshInterval: 100 * time.Millisecond,
				JitterPercent:   0,
				BackoffInitial:  100 * time.Millisecond,
				BackoffMax:      500 * time.Millisecond,
			},
			"minimax": {
				Name:            "minimax",
				BaseURL:         minimaxServer.URL(),
				Token:           "test-token-minimax",
				RefreshInterval: 100 * time.Millisecond,
				JitterPercent:   0,
				BackoffInitial:  100 * time.Millisecond,
				BackoffMax:      500 * time.Millisecond,
			},
			"zai": {
				Name:            "zai",
				BaseURL:         zaiServer.URL(),
				Token:           "test-token-zai",
				RefreshInterval: 100 * time.Millisecond,
				JitterPercent:   0,
				BackoffInitial:  100 * time.Millisecond,
				BackoffMax:      500 * time.Millisecond,
			},
			"zhipu": {
				Name:            "zhipu",
				BaseURL:         zhipuServer.URL(),
				Token:           "test-token-zhipu",
				RefreshInterval: 100 * time.Millisecond,
				JitterPercent:   0,
				BackoffInitial:  100 * time.Millisecond,
				BackoffMax:      500 * time.Millisecond,
			},
		},
	}

	// Create providers
	providers := []providertype.Provider{
		codex.NewWithClient("codex", cfg.Providers["codex"].BaseURL, cfg.Providers["codex"].Token, &http.Client{Timeout: 10 * time.Second}),
		kimi.NewWithClient("kimi", cfg.Providers["kimi"].BaseURL, cfg.Providers["kimi"].Token, &http.Client{Timeout: 10 * time.Second}),
		minimax.NewWithClient("minimax", cfg.Providers["minimax"].BaseURL, cfg.Providers["minimax"].Token, &http.Client{Timeout: 10 * time.Second}),
		monitorquota.NewZAIWithClient("zai", cfg.Providers["zai"].BaseURL, cfg.Providers["zai"].Token, &http.Client{Timeout: 10 * time.Second}),
		monitorquota.NewZhipuWithClient("zhipu", cfg.Providers["zhipu"].BaseURL, cfg.Providers["zhipu"].Token, &http.Client{Timeout: 10 * time.Second}),
	}

	// Create store and metrics
	s := store.NewWithConfig(cfg.Global.MaxStaleDuration)
	metrics := metrics.New()

	syncMgr := syncmanager.New(providers, s, cfg)

	builder := &app.Builder{
		Config:      cfg,
		Store:       s,
		Metrics:     metrics,
		SyncManager: syncMgr,
	}

	comp, err := builder.Build()
	require.NoError(t, err)

	// Start app in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- comp.Run(ctx)
	}()

	// Wait for first sync to complete (providers sync and update store)
	// Poll the store until we have all 5 platforms
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		platforms := s.Platforms()
		if len(platforms) == 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Make HTTP request to /api/v1/usage
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/api/v1/usage", apiPort), nil)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var usageResponses []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&usageResponses)
	require.NoError(t, err)

	// We should get 5 accounts (one per provider)
	assert.Len(t, usageResponses, 5, "expected 5 accounts")

	// Build a map by platform for easier assertions
	accounts := make(map[string]map[string]interface{})
	for _, r := range usageResponses {
		platform, ok := r["platform"].(string)
		require.True(t, ok, "platform should be a string")
		accounts[platform] = r
	}

	// Verify each provider exists and is healthy
	expectedProviders := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	for _, prov := range expectedProviders {
		account, ok := accounts[prov]
		require.True(t, ok, "expected provider %q in response", prov)
		assert.Equal(t, "healthy", account["status"], "provider %q should be healthy", prov)
		assert.NotNil(t, account["quotas"], "provider %q should have quotas", prov)
		assert.True(t, account["version"].(float64) > 0, "provider %q should have version > 0", prov)
	}

	// Clean up
	cancel()

	// Wait for app to stop
	select {
	case <-errCh:
		// App stopped
	case <-time.After(5 * time.Second):
		t.Log("timeout waiting for app to stop")
	}
}
