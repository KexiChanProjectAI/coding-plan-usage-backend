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
			"membership": map[string]interface{}{
				"level": map[string]interface{}{
					"name": "LEVEL_INTERMEDIATE",
				},
			},
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
	// usage: limit=1000, remaining=750, used=250, percent=25
	if tier5H.Used != 25 {
		t.Errorf("expected used 25 (250/1000*100), got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier5H.Total)
	}

	// Verify 1W tier from limits array
	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present from limits")
	}
	// limits: limit=500, remaining=200, used=300, percent=60
	if tier1W.Used != 60 {
		t.Errorf("expected used 60 (300/500*100), got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier1W.Total)
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
	// limit=100, remaining=40, used=60, percent=60
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier5H.Total)
	}
	if tier5H.Used != 60 {
		t.Errorf("expected used 60 (60/100*100), got %d", tier5H.Used)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present from limits")
	}
	// limit=500, remaining=300, used=200, percent=40
	if tier1W.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier1W.Total)
	}
	if tier1W.Used != 40 {
		t.Errorf("expected used 40 (200/500*100), got %d", tier1W.Used)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be present from limits")
	}
	// limit=2000, remaining=1500, used=500, percent=25
	if tier1M.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier1M.Total)
	}
	if tier1M.Used != 25 {
		t.Errorf("expected used 25 (500/2000*100), got %d", tier1M.Used)
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
	// All 3 tiers exist due to backfill
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 tiers (usage + 2 backfilled), got %d tiers", len(snapshot.Quotas))
	}

	// The 5H tier should have Total=100 from usage (percent semantics), not 999 from parallel
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Total != 100 {
		t.Errorf("tier %s expected total=100 (usage percent), got %d", domain.Tier5H, tier5H.Total)
	}
	if tier5H.Used != 50 {
		t.Errorf("tier %s expected used=50 (500/1000*100), got %d", domain.Tier5H, tier5H.Used)
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

	// Verify the tier has usage values, not parallel values
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	// usage: limit=1000, remaining=500, percent=50
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier5H.Total)
	}
	if tier5H.Used != 50 {
		t.Errorf("expected used 50 (500/1000*100), got %d", tier5H.Used)
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

	// BackfillCanonicalTiers adds all 3 canonical tiers with Used=0, Total=100
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 tiers after backfill, got %d tiers", len(snapshot.Quotas))
	}

	// Verify all three canonical tiers are present with default values
	for _, tier := range []domain.Tier{domain.Tier5H, domain.Tier1W, domain.Tier1M} {
		qt, ok := snapshot.Quotas[tier]
		if !ok {
			t.Errorf("expected tier %s to be present after backfill", tier)
			continue
		}
		if qt.Used != 0 {
			t.Errorf("tier %s expected used=0, got %d", tier, qt.Used)
		}
		if qt.Total != 100 {
			t.Errorf("tier %s expected total=100, got %d", tier, qt.Total)
		}
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

	// 120 minutes = 2 hours and 2 hours both map to 5H
	// Second limit (200, remaining=100, used=100, percent=50) overwrites first
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 (percent semantics), got %d", tier5H.Total)
	}
	if tier5H.Used != 50 {
		t.Errorf("expected used 50 (100/200*100), got %d", tier5H.Used)
	}
}

func TestKimiBackfillsMissingTiers(t *testing.T) {
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
					"limit":     "1000",
					"remaining": "250",
					"resetTime": "2025-04-28T15:00:00Z",
				},
			},
		},
	})

	adapter := New("kimi", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Upstream only returned 5H limit, but all three canonical tiers must be present
	if len(snapshot.Quotas) != 3 {
		t.Fatalf("expected 3 tiers after backfill, got %d tiers", len(snapshot.Quotas))
	}

	// Verify 5H tier has actual data from upstream
	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	// limit=1000, remaining=250, used=750, percent=75
	if tier5H.Used != 75 {
		t.Errorf("5H: expected used 75 (750/1000*100), got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("5H: expected total 100, got %d", tier5H.Total)
	}
	if tier5H.ResetAt.IsZero() {
		t.Error("5H: expected non-zero ResetAt")
	}

	// Verify 1W tier was backfilled
	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be backfilled")
	}
	if tier1W.Used != 0 {
		t.Errorf("1W: expected used 0 (backfill default), got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("1W: expected total 100 (backfill default), got %d", tier1W.Total)
	}
	if !tier1W.ResetAt.IsZero() {
		t.Errorf("1W: expected zero ResetAt (backfill default), got %v", tier1W.ResetAt)
	}

	// Verify 1M tier was backfilled
	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be backfilled")
	}
	if tier1M.Used != 0 {
		t.Errorf("1M: expected used 0 (backfill default), got %d", tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("1M: expected total 100 (backfill default), got %d", tier1M.Total)
	}
	if !tier1M.ResetAt.IsZero() {
		t.Errorf("1M: expected zero ResetAt (backfill default), got %v", tier1M.ResetAt)
	}
}