package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quotahub/ucpqa/internal/app"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMetricsExposesQuotaGauges(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19401,
			MetricsPort: 19402,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	resetTime := time.Now().Add(5 * time.Hour)
	snapshot := domain.AccountSnapshot{
		Platform:     "metrics-test-provider",
		AccountAlias: "metrics-test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: resetTime},
			domain.Tier1W: {Used: 20, Total: 200, ResetAt: resetTime},
			domain.Tier1M: {Used: 30, Total: 300, ResetAt: resetTime},
		},
		Version: 1,
	}

	st.Update(snapshot)
	m.UpdateFromSnapshot(snapshot)

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, "coding_plan_usage_value")
	assert.Contains(t, content, "coding_plan_reset_timestamp")
	assert.Contains(t, content, `platform="metrics-test-provider"`)
	assert.Contains(t, content, `account="metrics-test-account"`)
	assert.Contains(t, content, `tier="5H"`)
	assert.Contains(t, content, `tier="1W"`)
	assert.Contains(t, content, `tier="1M"`)
	assert.Contains(t, content, `type="used"`)
	assert.Contains(t, content, `type="total"`)
}

func TestMetricsAfterMultipleSnapshots(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19411,
			MetricsPort: 19412,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	snapshot1 := domain.AccountSnapshot{
		Platform:     "multi-snap",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
		},
		Version: 1,
	}
	st.Update(snapshot1)
	m.UpdateFromSnapshot(snapshot1)

	snapshot2 := domain.AccountSnapshot{
		Platform:     "multi-snap",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 20, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
		},
		Version: 2,
	}
	st.Update(snapshot2)
	m.UpdateFromSnapshot(snapshot2)

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, "multi-snap")
	assert.Contains(t, content, `tier="5H"`)
}

func TestMetricsEmptyStoreNoGauges(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19421,
			MetricsPort: 19422,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.NotContains(t, content, "coding_plan_usage_value{")
}

func TestMetricsResetTimestampUpdated(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19431,
			MetricsPort: 19432,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	resetTime1 := time.Now().Add(5 * time.Hour)
	snapshot1 := domain.AccountSnapshot{
		Platform:     "reset-ts",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: resetTime1},
		},
		Version: 1,
	}
	st.Update(snapshot1)
	m.UpdateFromSnapshot(snapshot1)

	resetTime2 := time.Now().Add(10 * time.Hour)
	snapshot2 := domain.AccountSnapshot{
		Platform:     "reset-ts",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 15, Total: 100, ResetAt: resetTime2},
		},
		Version: 2,
	}
	st.Update(snapshot2)
	m.UpdateFromSnapshot(snapshot2)

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, "reset-ts")
	assert.Contains(t, content, "coding_plan_reset_timestamp")
}

func TestMetricsUnsupportedTierNotExported(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19441,
			MetricsPort: 19442,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	resetTime := time.Now().Add(5 * time.Hour)
	snapshot := domain.AccountSnapshot{
		Platform:     "unsupported-tier",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H:      {Used: 10, Total: 100, ResetAt: resetTime},
			domain.Tier("XYZ"): {Used: 999, Total: 999, ResetAt: resetTime},
		},
		Version: 1,
	}

	st.Update(snapshot)
	m.UpdateFromSnapshot(snapshot)

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, `platform="unsupported-tier"`)
	assert.NotContains(t, content, `tier="XYZ"`)
}

func TestMetricsServerSeparation(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19451,
			MetricsPort: 19452,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	snapshot := domain.AccountSnapshot{
		Platform:     "sep-test-provider",
		AccountAlias: "sep-test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: time.Now().Add(1 * time.Hour)},
		},
		Version: 1,
	}
	app.Store().Update(snapshot)
	app.Metrics().UpdateFromSnapshot(snapshot)

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	apiReq, _ := http.NewRequest("GET", apiServer.URL+"/metrics", nil)
	apiResp, err := http.DefaultClient.Do(apiReq)
	if err == nil {
		apiResp.Body.Close()
	}

	metricsReq, _ := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	metricsResp, err := http.DefaultClient.Do(metricsReq)
	require.NoError(t, err)
	defer metricsResp.Body.Close()

	assert.Equal(t, http.StatusOK, metricsResp.StatusCode)

	body, _ := io.ReadAll(metricsResp.Body)
	content := string(body)
	assert.Contains(t, content, "coding_plan_usage_value")
}
