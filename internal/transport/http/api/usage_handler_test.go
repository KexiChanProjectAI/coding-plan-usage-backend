package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageHandlerReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := store.New()
	maxStaleDuration := 1 * time.Hour

	snap := domain.AccountSnapshot{
		Platform:     "testplatform",
		AccountAlias: "testalias",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 100, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
			domain.Tier1W: {Used: 200, Total: 100, ResetAt: time.Now().Add(7 * 24 * time.Hour)},
		},
		LastSync: time.Now(),
		Version:  1,
	}
	s.Update(snap)

	handler := NewUsageHandler(s, maxStaleDuration)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []UsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)

	require.Len(t, responses, 1)
	resp := responses[0]

	assert.Equal(t, "testplatform", resp.Platform)
	assert.Equal(t, "testalias", resp.AccountAlias)
	assert.Equal(t, int64(1), resp.Version)
	assert.Equal(t, domain.StatusHealthy, resp.Status)
	assert.NotNil(t, resp.Quotas)
	assert.Equal(t, int64(100), resp.Quotas[domain.Tier5H].Used)
	assert.Equal(t, int64(100), resp.Quotas[domain.Tier5H].Total)
}

func TestUsageHandlerInitializingState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := store.New()
	maxStaleDuration := 1 * time.Hour

	handler := NewUsageHandler(s, maxStaleDuration)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []UsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)

	require.Len(t, responses, 1)
	resp := responses[0]

	assert.Equal(t, "", resp.Platform)
	assert.Equal(t, int64(0), resp.Version)
	assert.Equal(t, domain.StatusInitializing, resp.Status)
}

func TestUsageHandlerDegradedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := store.New()
	maxStaleDuration := 1 * time.Hour

	staleResetAt := time.Now().Add(-2 * time.Hour)
	snap := domain.AccountSnapshot{
		Platform:     "staleplatform",
		AccountAlias: "stalelias",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 500, Total: 100, ResetAt: staleResetAt},
		},
		LastSync: time.Now(),
		Version:  1,
	}
	s.Update(snap)

	handler := NewUsageHandler(s, maxStaleDuration)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []UsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)

	require.Len(t, responses, 1)
	resp := responses[0]

	assert.Equal(t, domain.StatusDegraded, resp.Status)
}

func TestUsageHandlerMultiplePlatforms(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := store.New()
	maxStaleDuration := 1 * time.Hour

	snap1 := domain.AccountSnapshot{
		Platform:     "platform1",
		AccountAlias: "alias1",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 100, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
		},
		LastSync: time.Now(),
		Version:  1,
	}
	snap2 := domain.AccountSnapshot{
		Platform:     "platform2",
		AccountAlias: "alias2",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier1W: {Used: 200, Total: 100, ResetAt: time.Now().Add(7 * 24 * time.Hour)},
		},
		LastSync: time.Now(),
		Version:  1,
	}
	s.Update(snap1)
	s.Update(snap2)

	handler := NewUsageHandler(s, maxStaleDuration)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []UsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)

	assert.Len(t, responses, 2)

	platforms := make(map[string]domain.Status)
	for _, resp := range responses {
		platforms[resp.Platform] = resp.Status
	}

	assert.Equal(t, domain.StatusHealthy, platforms["platform1"])
	assert.Equal(t, domain.StatusHealthy, platforms["platform2"])
}

func TestUsageHandlerUnsupportedTiersOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := store.New()
	maxStaleDuration := 1 * time.Hour

	snap := domain.AccountSnapshot{
		Platform:     "testplatform",
		AccountAlias: "testalias",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 100, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
			domain.Tier1W: {Used: 0, Total: 100, ResetAt: time.Time{}},
			domain.Tier1M: {Used: 0, Total: 100, ResetAt: time.Time{}},
		},
		LastSync: time.Now(),
		Version:  1,
	}
	s.Update(snap)

	handler := NewUsageHandler(s, maxStaleDuration)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []UsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)

	require.Len(t, responses, 1)
	resp := responses[0]

	_, has5H := resp.Quotas[domain.Tier5H]
	_, has1W := resp.Quotas[domain.Tier1W]
	_, has1M := resp.Quotas[domain.Tier1M]
	_, hasUnsupported := resp.Quotas["UNSUPPORTED"]

	assert.True(t, has5H, "5H tier should be present")
	assert.True(t, has1W, "1W tier should be present (backfilled with zero values)")
	assert.True(t, has1M, "1M tier should be present (backfilled with zero values)")
	assert.False(t, hasUnsupported, "unsupported tier should not be present")
}