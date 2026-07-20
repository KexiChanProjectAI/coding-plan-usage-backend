package cpa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestFetchAveragesSuccessfulAccounts(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	require.NoError(t, server.SetResponse("/auth-files", http.StatusOK, map[string]interface{}{
		"files": []map[string]interface{}{
			{
				"auth_index":  "idx-1",
				"name":        "acct1.json",
				"type":        "codex",
				"label":       "acct1@example.com",
				"status":      "active",
				"disabled":    false,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "aaaa-bbbb-cccc",
					"plan_type":          "plus",
				},
			},
			{
				"auth_index":  "idx-2",
				"name":        "acct2.json",
				"type":        "codex",
				"email":       "acct2@example.com",
				"status":      "active",
				"disabled":    false,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "dddd-eeee-ffff",
					"plan_type":          "plus",
				},
			},
		},
	}))

	require.NoError(t, server.SetResponse("/api-call", http.StatusOK, map[string]interface{}{
		"status_code": http.StatusOK,
		"body":        mustMarshal(usageBody(30, 10)), // 70% and 90% remaining -> worst 30% used -> 70% remaining
	}))

	adapter := NewWithClient("cpa", server.URL(), "mgmt-key", server.HTTPClient())
	adapter.stagger = 0

	snapshot, err := adapter.Fetch(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "cpa", snapshot.Platform)
	assert.Equal(t, "openai-pool", snapshot.AccountAlias)

	// Both accounts have 70% effective remaining (worst window used 30%).
	// Average remaining = 70, so used = 30.
	for _, tier := range domain.AllSupportedTiers {
		q, ok := snapshot.Quotas[tier]
		require.True(t, ok, "expected tier %s", tier)
		assert.Equal(t, int64(30), q.Used, "tier %s used", tier)
		assert.Equal(t, int64(100), q.Total, "tier %s total", tier)
		assert.False(t, q.ResetAt.IsZero(), "tier %s reset_at", tier)
	}
}

func TestFetchSkipsNonCodexAndDisabledAccounts(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	require.NoError(t, server.SetResponse("/auth-files", http.StatusOK, map[string]interface{}{
		"files": []map[string]interface{}{
			{
				"auth_index":  "idx-1",
				"name":        "claude.json",
				"type":        "claude",
				"status":      "active",
				"disabled":    false,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "should-not-be-queried",
				},
			},
			{
				"auth_index":  "idx-2",
				"name":        "disabled.json",
				"type":        "codex",
				"status":      "active",
				"disabled":    true,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "disabled-id",
				},
			},
			{
				"auth_index":  "idx-3",
				"name":        "badstatus.json",
				"type":        "codex",
				"status":      "expired",
				"disabled":    false,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "badstatus-id",
				},
			},
			{
				"auth_index":  "idx-4",
				"name":        "good.json",
				"type":        "codex",
				"label":       "good@example.com",
				"status":      "active",
				"disabled":    false,
				"unavailable": false,
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "good-id",
					"plan_type":          "plus",
				},
			},
		},
	}))

	require.NoError(t, server.SetResponse("/api-call", http.StatusOK, map[string]interface{}{
		"status_code": http.StatusOK,
		"body":        mustMarshal(usageBody(50, 0)), // 50% remaining
	}))

	adapter := NewWithClient("cpa", server.URL(), "mgmt-key", server.HTTPClient())
	adapter.stagger = 0

	snapshot, err := adapter.Fetch(context.Background())
	require.NoError(t, err)

	q := snapshot.Quotas[domain.Tier5H]
	assert.Equal(t, int64(50), q.Used)
}

func TestFetchSkipsAccountsMissingRequiredFields(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	require.NoError(t, server.SetResponse("/auth-files", http.StatusOK, map[string]interface{}{
		"files": []map[string]interface{}{
			{
				"name":     "no-auth-index.json",
				"type":     "codex",
				"status":   "active",
				"disabled": false,
				"id_token": map[string]interface{}{"chatgpt_account_id": "id"},
			},
			{
				"auth_index": "idx-1",
				"name":       "no-account-id.json",
				"type":       "codex",
				"status":     "active",
				"disabled":   false,
				"id_token":   map[string]interface{}{},
			},
		},
	}))

	adapter := NewWithClient("cpa", server.URL(), "mgmt-key", server.HTTPClient())
	adapter.stagger = 0

	_, err := adapter.Fetch(context.Background())
	require.Error(t, err)
	assert.True(t, provider.IsFetchFailure(err))
	assert.Contains(t, err.Error(), "no queryable OpenAI Codex accounts found")
}

func TestFetchPerAccountFailureIgnored(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth-files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mustMarshal(map[string]interface{}{
			"files": []map[string]interface{}{
				{
					"auth_index": "idx-1",
					"name":       "good.json",
					"type":       "codex",
					"status":     "active",
					"id_token": map[string]interface{}{
						"chatgpt_account_id": "good-id",
					},
				},
				{
					"auth_index": "idx-2",
					"name":       "bad.json",
					"type":       "codex",
					"status":     "active",
					"id_token": map[string]interface{}{
						"chatgpt_account_id": "bad-id",
					},
				},
			},
		})))
	})

	mux.HandleFunc("/api-call", func(w http.ResponseWriter, r *http.Request) {
		var req apiCallRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.AuthIndex == "idx-1" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mustMarshal(map[string]interface{}{
				"status_code": http.StatusOK,
				"body":        mustMarshal(usageBody(25, 0)), // 75% remaining
			})))
			return
		}
		// idx-2 returns upstream 401
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mustMarshal(map[string]interface{}{
			"status_code": http.StatusUnauthorized,
			"body":        "",
		})))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	adapter := NewWithClient("cpa", server.URL, "mgmt-key", server.Client())
	adapter.stagger = 0

	snapshot, err := adapter.Fetch(context.Background())
	require.NoError(t, err)

	q := snapshot.Quotas[domain.Tier5H]
	assert.Equal(t, int64(25), q.Used) // 100 - 75 = 25
}

func TestFetchAllAccountsFail(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	require.NoError(t, server.SetResponse("/auth-files", http.StatusOK, map[string]interface{}{
		"files": []map[string]interface{}{
			{
				"auth_index": "idx-1",
				"name":       "bad.json",
				"type":       "codex",
				"status":     "active",
				"id_token": map[string]interface{}{
					"chatgpt_account_id": "bad-id",
				},
			},
		},
	}))

	require.NoError(t, server.SetResponse("/api-call", http.StatusOK, map[string]interface{}{
		"status_code": http.StatusUnauthorized,
		"body":        "",
	}))

	adapter := NewWithClient("cpa", server.URL(), "mgmt-key", server.HTTPClient())
	adapter.stagger = 0

	_, err := adapter.Fetch(context.Background())
	require.Error(t, err)
	assert.True(t, provider.IsFetchFailure(err))
}

func TestFetchAuthFilesFailure(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	require.NoError(t, server.SetResponse("/auth-files", http.StatusInternalServerError, ""))

	adapter := NewWithClient("cpa", server.URL(), "mgmt-key", server.HTTPClient())
	_, err := adapter.Fetch(context.Background())
	require.Error(t, err)
	assert.True(t, provider.IsUpstreamRejection(err))
}

func TestStaggerBetweenRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth-files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mustMarshal(map[string]interface{}{
			"files": []map[string]interface{}{
				{
					"auth_index": "idx-1",
					"name":       "a.json",
					"type":       "codex",
					"status":     "active",
					"id_token":   map[string]interface{}{"chatgpt_account_id": "a"},
				},
				{
					"auth_index": "idx-2",
					"name":       "b.json",
					"type":       "codex",
					"status":     "active",
					"id_token":   map[string]interface{}{"chatgpt_account_id": "b"},
				},
			},
		})))
	})

	var callTimes []time.Time
	mux.HandleFunc("/api-call", func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mustMarshal(map[string]interface{}{
			"status_code": http.StatusOK,
			"body":        mustMarshal(usageBody(0, 0)),
		})))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	adapter := NewWithClient("cpa", server.URL, "mgmt-key", server.Client())
	adapter.stagger = 100 * time.Millisecond

	_, err := adapter.Fetch(context.Background())
	require.NoError(t, err)

	require.Len(t, callTimes, 2)
	gap := callTimes[1].Sub(callTimes[0])
	assert.True(t, gap >= 90*time.Millisecond, "expected stagger gap, got %v", gap)
}

func TestEffectiveRemainingBlocked(t *testing.T) {
	cases := []struct {
		name     string
		resp     *usageResponse
		expected int
	}{
		{
			name: "limit_reached",
			resp: &usageResponse{
				RateLimit: &rateLimit{LimitReached: true, Allowed: true},
			},
			expected: 0,
		},
		{
			name: "allowed_false",
			resp: &usageResponse{
				RateLimit: &rateLimit{Allowed: false},
			},
			expected: 0,
		},
		{
			name: "credits_override",
			resp: &usageResponse{
				RateLimit: &rateLimit{LimitReached: true, Allowed: true},
				Credits:   &credits{HasCredits: true},
			},
			expected: 100,
		},
		{
			name: "spend_control_reached",
			resp: &usageResponse{
				RateLimit:    &rateLimit{Allowed: true},
				SpendControl: &spendControl{Reached: true},
			},
			expected: 0,
		},
		{
			name: "no_windows",
			resp: &usageResponse{
				RateLimit: &rateLimit{Allowed: true},
			},
			expected: 100,
		},
		{
			name: "single_window",
			resp: &usageResponse{
				RateLimit: &rateLimit{
					Allowed: true,
					Primary: &window{UsedPercent: 23},
				},
			},
			expected: 77,
		},
		{
			name: "worst_window",
			resp: &usageResponse{
				RateLimit: &rateLimit{
					Allowed:   true,
					Primary:   &window{UsedPercent: 23},
					Secondary: &window{UsedPercent: 45},
				},
			},
			expected: 55,
		},
		{
			name: "overuse_clamped",
			resp: &usageResponse{
				RateLimit: &rateLimit{
					Allowed: true,
					Primary: &window{UsedPercent: 150},
				},
			},
			expected: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, effectiveRemaining(tc.resp))
		})
	}
}

func TestClampPercent(t *testing.T) {
	assert.Equal(t, 0, clampPercent(-5))
	assert.Equal(t, 0, clampPercent(0))
	assert.Equal(t, 50, clampPercent(50))
	assert.Equal(t, 100, clampPercent(100))
	assert.Equal(t, 100, clampPercent(150))
}

func TestSleepCtxRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepCtx(ctx, 10*time.Millisecond)
	assert.Equal(t, context.Canceled, err)
}

func usageBody(sessionUsed, weeklyUsed int) map[string]interface{} {
	return map[string]interface{}{
		"plan_type": "plus",
		"rate_limit": map[string]interface{}{
			"allowed":       true,
			"limit_reached": false,
			"primary_window": map[string]interface{}{
				"used_percent":         sessionUsed,
				"limit_window_seconds": 18000,
				"reset_after_seconds":  10000,
				"reset_at":             1781276043,
			},
			"secondary_window": map[string]interface{}{
				"used_percent":         weeklyUsed,
				"limit_window_seconds": 604800,
				"reset_after_seconds":  300000,
				"reset_at":             1781622947,
			},
		},
		"credits": map[string]interface{}{
			"has_credits": false,
			"unlimited":   false,
		},
	}
}

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal failed: %v", err))
	}
	return string(b)
}
