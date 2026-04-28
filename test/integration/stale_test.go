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

func TestStaleExpiryOmitsTier(t *testing.T) {
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

	now := time.Now()
	staleResetAt := now.Add(-48 * time.Hour)
	freshResetAt := now.Add(5 * time.Hour)

	kimiResetAt := freshResetAt.Format(time.RFC3339)
	kimiResp := map[string]interface{}{
		"usage": map[string]string{
			"limit":     "1000",
			"remaining": "600",
			"resetTime": kimiResetAt,
		},
	}
	err := kimiServer.SetResponse("/coding/v1/usages", 200, kimiResp)
	require.NoError(t, err)

	minimaxResp := map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 1000,
				"current_interval_usage_count": 700,
				"end_time":                     freshResetAt.UnixMilli(),
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
		},
	}
	err = minimaxServer.SetResponse("/v1/api/openplatform/coding_plan/remains", 200, minimaxResp)
	require.NoError(t, err)

	zaiResp := map[string]interface{}{
		"code":    200,
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"unit":          3,
					"usage":         450,
					"currentValue":  1000,
					"nextResetTime": freshResetAt.UnixMilli(),
				},
			},
		},
	}
	err = zaiServer.SetResponse("/api/monitor/usage/quota/limit", 200, zaiResp)
	require.NoError(t, err)

	zhipuResp := map[string]interface{}{
		"code":    200,
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"unit":          5,
					"usage":         1500,
					"currentValue":  5000,
					"nextResetTime": freshResetAt.UnixMilli(),
				},
			},
		},
	}
	err = zhipuServer.SetResponse("/api/monitor/usage/quota/limit", 200, zhipuResp)
	require.NoError(t, err)

	codexResp := map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         45.5,
				"limit_window_seconds": 18000,
				"reset_at":             staleResetAt.Unix(),
			},
		},
	}
	err = codexServer.SetResponse("/wham/usage", 200, codexResp)
	require.NoError(t, err)

	apiPort := 18082
	metricsPort := 18092
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

	providers := []providertype.Provider{
		codex.NewWithClient("codex", cfg.Providers["codex"].BaseURL, cfg.Providers["codex"].Token, &http.Client{Timeout: 10 * time.Second}),
		kimi.NewWithClient("kimi", cfg.Providers["kimi"].BaseURL, cfg.Providers["kimi"].Token, &http.Client{Timeout: 10 * time.Second}),
		minimax.NewWithClient("minimax", cfg.Providers["minimax"].BaseURL, cfg.Providers["minimax"].Token, &http.Client{Timeout: 10 * time.Second}),
		monitorquota.NewZAIWithClient("zai", cfg.Providers["zai"].BaseURL, cfg.Providers["zai"].Token, &http.Client{Timeout: 10 * time.Second}),
		monitorquota.NewZhipuWithClient("zhipu", cfg.Providers["zhipu"].BaseURL, cfg.Providers["zhipu"].Token, &http.Client{Timeout: 10 * time.Second}),
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- comp.Run(ctx)
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		platforms := s.Platforms()
		if len(platforms) == 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

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

	accounts := make(map[string]map[string]interface{})
	for _, r := range usageResponses {
		platform, ok := r["platform"].(string)
		require.True(t, ok)
		accounts[platform] = r
	}

	assert.Equal(t, "healthy", accounts["codex"]["status"], "codex should remain healthy after stale tier omission")
	// With backfill, codex still has quotas (backfilled 1W/1M with zero ResetAt are preserved by omitStaleQuotas)
	// but the stale 5H tier is omitted
	codexQuotas, hasCodexQuotas := accounts["codex"]["quotas"].(map[string]interface{})
	assert.True(t, hasCodexQuotas, "codex should have quotas from backfill")
	if hasCodexQuotas {
		_, has5H := codexQuotas["5H"]
		assert.False(t, has5H, "codex 5H tier should be omitted (stale)")
		_, has1W := codexQuotas["1W"]
		assert.True(t, has1W, "codex 1W tier should be present (backfilled with zero ResetAt)")
		_, has1M := codexQuotas["1M"]
		assert.True(t, has1M, "codex 1M tier should be present (backfilled with zero ResetAt)")
	}
	for _, prov := range []string{"kimi", "minimax", "zai", "zhipu"} {
		assert.Equal(t, "healthy", accounts[prov]["status"], "provider %q should be healthy", prov)
	}

	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Log("timeout waiting for app to stop")
	}
}
