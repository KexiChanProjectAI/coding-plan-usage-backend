package kimi

import (
	"context"
	"net/http"
	"testing"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestFetchQuotaSuccess(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"userId":   "user123",
			"region":   "us",
			"membership": "premium",
		},
		"usage": map[string]interface{}{
			"limit":     "1000",
			"remaining": "750",
			"resetTime": "2025-04-28T00:00:00Z", // ~5 hours from now
		},
		"limits": []map[string]interface{}{
			{
				"window": map[string]interface{}{
					"duration": 7,
					"timeUnit": "TIME_UNIT_DAY",
				},
				"detail": map[string]interface{}{
					"limit":     "500",
					"remaining": "200",
					"resetTime": "2025-05-04T00:00:00Z",
				},
			},
		},
		"parallel": map[string]interface{}{
			"limit": "10",
		},
		"totalQuota": map[string]interface{}{
			"limit":     "10000",
			"remaining": "7500",
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "kimi" {
		t.Errorf("expected platform kimi, got %s", snapshot.Platform)
	}

	// Verify 5H tier from usage section
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Used != 250 {
		t.Errorf("expected used 250 (1000-750), got %d", tier5H.Used)
	}
	if tier5H.Total != 1000 {
		t.Errorf("expected total 1000, got %d", tier5H.Total)
	}

	// Verify 1W tier from limits array
	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present from limits")
	}
	if tier1W.Total != 500 {
		t.Errorf("expected total 500, got %d", tier1W.Total)
	}
}

func TestLimitsArrayMapping(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"limits": []map[string]interface{}{
			{
				"window": map[string]interface{}{
					"duration": 5,
					"timeUnit": "TIME_UNIT_HOUR",
				},
				"detail": map[string]interface{}{
					"limit":     "100",
					"remaining": "40",
					"resetTime": "2025-04-27T18:00:00Z",
				},
			},
			{
				"window": map[string]interface{}{
					"duration": 7,
					"timeUnit": "TIME_UNIT_DAY",
				},
				"detail": map[string]interface{}{
					"limit":     "500",
					"remaining": "300",
					"resetTime": "2025-05-04T00:00:00Z",
				},
			},
			{
				"window": map[string]interface{}{
					"duration": 1,
					"timeUnit": "TIME_UNIT_MONTH",
				},
				"detail": map[string]interface{}{
					"limit":     "2000",
					"remaining": "1500",
					"resetTime": "2025-05-27T00:00:00Z",
				},
			},
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present from limits")
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present from limits")
	}
	if tier1W.Total != 500 {
		t.Errorf("expected total 500, got %d", tier1W.Total)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be present from limits")
	}
	if tier1M.Total != 2000 {
		t.Errorf("expected total 2000, got %d", tier1M.Total)
	}
}

func TestParallelLimitExcluded(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"usage": map[string]interface{}{
			"limit":     "1000",
			"remaining": "500",
			"resetTime": "2025-04-27T20:00:00Z",
		},
		"parallel": map[string]interface{}{
			"limit": "999", // This high value should NOT appear as a quota
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// parallel.limit should not create a separate tier
	if len(snapshot.Quotas) > 1 {
		t.Errorf("expected only usage-based tier, got %d tiers", len(snapshot.Quotas))
	}

	// The tier should have Total=1000 from usage, not 999 from parallel
	for tier, quota := range snapshot.Quotas {
		if quota.Total == 999 {
			t.Errorf("tier %s has total 999 from parallel.limit - should not be included", tier)
		}
	}
}

func TestTotalQuotaExcluded(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"usage": map[string]interface{}{
			"limit":     "1000",
			"remaining": "500",
			"resetTime": "2025-04-27T20:00:00Z",
		},
		"totalQuota": map[string]interface{}{
			"limit":     "99999", // This should NOT appear as a tier
			"remaining": "88888",
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the tier has usage values, not totalQuota values
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Total == 99999 {
		t.Error("tier has total from totalQuota - should not be included")
	}
	if tier5H.Total != 1000 {
		t.Errorf("expected total 1000 from usage, got %d", tier5H.Total)
	}
}

func TestInvalidNumericString(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"usage": map[string]interface{}{
			"limit":     "not-a-number",
			"remaining": "500",
			"resetTime": "2025-04-27T20:00:00Z",
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for invalid numeric string")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestInvalidResetTimeFormat(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"usage": map[string]interface{}{
			"limit":     "1000",
			"remaining": "500",
			"resetTime": "invalid-time-format",
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for invalid reset time format")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestHTTPError(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusInternalServerError, nil)

	adapter := New("kimi", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for HTTP error status")
	}

	if !provider.IsUpstreamRejection(err) {
		t.Errorf("expected upstream rejection error, got %T", err)
	}
}

func TestParseFailure(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, "invalid json{")

	adapter := New("kimi", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for parse failure")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestProviderName(t *testing.T) {
	adapter := New("kimi", "https://example.com", "token")
	if adapter.ProviderName() != "kimi" {
		t.Errorf("expected provider name 'kimi', got %s", adapter.ProviderName())
	}
}

func TestNoUsageOrLimits(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"userId": "user123",
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshot.Quotas) != 0 {
		t.Errorf("expected empty quotas, got %d tiers", len(snapshot.Quotas))
	}
}

func TestDurationTimeUnitConversions(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/coding/v1/usages", http.StatusOK, map[string]interface{}{
		"limits": []map[string]interface{}{
			{
				"window": map[string]interface{}{
					"duration": 120,
					"timeUnit": "TIME_UNIT_MINUTE",
				},
				"detail": map[string]interface{}{
					"limit":     "100",
					"remaining": "50",
					"resetTime": "2025-04-27T18:00:00Z",
				},
			},
			{
				"window": map[string]interface{}{
					"duration": 2,
					"timeUnit": "TIME_UNIT_HOUR",
				},
				"detail": map[string]interface{}{
					"limit":     "200",
					"remaining": "100",
					"resetTime": "2025-04-27T18:00:00Z",
				},
			},
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 120 minutes = 2 hours, should map to 5H
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	// Should have both limits combined (or at least one present)
	if tier5H.Total == 0 {
		t.Error("expected non-zero total for 5H tier")
	}
}