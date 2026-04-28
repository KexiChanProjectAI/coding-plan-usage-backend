package codex

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

	usedPercent := 30.0
	resetAt := int64(1745270400)
	limitWindowSeconds := int64(18000)

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"plan_type": "pro",
		"rate_limit": map[string]interface{}{
			"allowed":      true,
			"limit_reached": false,
			"primary_window": map[string]interface{}{
				"used_percent":         usedPercent,
				"limit_window_seconds": limitWindowSeconds,
				"reset_at":             resetAt,
			},
			"secondary_window": map[string]interface{}{
				"used_percent":         25.0,
				"limit_window_seconds": int64(604800),
				"reset_at":             int64(1745270400),
			},
		},
		"additional_rate_limits": []map[string]interface{}{
			{
				"metered_feature": "tokens",
				"rate_limit": map[string]interface{}{
					"used_percent":         50.0,
					"limit_window_seconds": int64(2592000),
					"reset_at":             int64(1745270400),
				},
			},
		},
		"credits": map[string]interface{}{
			"has_credits": true,
			"unlimited":   false,
			"balance":     "100.00",
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "codex" {
		t.Errorf("expected platform codex, got %s", snapshot.Platform)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	expectedUsed := int64(30)
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used %d, got %d", expectedUsed, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present")
	}
	expectedUsed1W := int64(25)
	if tier1W.Used != expectedUsed1W {
		t.Errorf("expected used %d, got %d", expectedUsed1W, tier1W.Used)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be present from additional_rate_limits")
	}
	expectedUsed1M := int64(50)
	if tier1M.Used != expectedUsed1M {
		t.Errorf("expected used %d, got %d", expectedUsed1M, tier1M.Used)
	}
}

func TestPrimaryWindowOnly(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usedPercent := 45.0
	resetAt := int64(1745270400)

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         usedPercent,
				"limit_window_seconds": int64(18000),
				"reset_at":             resetAt,
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}

	// With backfill, 1W and 1M are always present (with zero values)
	_, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be present (backfilled)")
	}
	_, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be present (backfilled)")
	}

	// Verify the backfilled tiers have zero values
	if snapshot.Quotas[domain.Tier1W].Used != 0 || snapshot.Quotas[domain.Tier1W].Total != 100 {
		t.Errorf("expected 1W backfilled with Used=0, Total=100, got Used=%d, Total=%d",
			snapshot.Quotas[domain.Tier1W].Used, snapshot.Quotas[domain.Tier1W].Total)
	}
	if snapshot.Quotas[domain.Tier1M].Used != 0 || snapshot.Quotas[domain.Tier1M].Total != 100 {
		t.Errorf("expected 1M backfilled with Used=0, Total=100, got Used=%d, Total=%d",
			snapshot.Quotas[domain.Tier1M].Used, snapshot.Quotas[domain.Tier1M].Total)
	}
}

func TestUnmappableAdditionalLimit(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         10.0,
				"limit_window_seconds": int64(18000),
				"reset_at":             int64(1745270400),
			},
		},
		"additional_rate_limits": []map[string]interface{}{
			{
				"metered_feature": "特殊功能",
				"rate_limit": map[string]interface{}{
					"used_percent":         50.0,
					"limit_window_seconds": int64(99999),
					"reset_at":             int64(1745270400),
				},
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}

	// With backfill, all 3 canonical tiers are present
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 tiers (5H + backfilled 1W + backfilled 1M), got %d", len(snapshot.Quotas))
	}
}

func TestHTTPError(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/wham/usage", http.StatusInternalServerError, nil)

	adapter := New("codex", server.URL(), "test-token")
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

	server.SetResponse("/wham/usage", http.StatusOK, "invalid json{")

	adapter := New("codex", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for parse failure")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestProviderName(t *testing.T) {
	adapter := New("codex", "https://example.com", "token")
	if adapter.ProviderName() != "codex" {
		t.Errorf("expected provider name 'codex', got %s", adapter.ProviderName())
	}
}

func TestNullUsedPercentWithWindow(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	resetAt := int64(1745270400)

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         nil,
				"limit_window_seconds": int64(18000),
				"reset_at":             resetAt,
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present even with null used_percent")
	}
	if tier5H.Used != 0 {
		t.Errorf("expected used 0 when used_percent is null, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 when used_percent is null with window (percentage contract), got %d", tier5H.Total)
	}

	// 1W and 1M are backfilled with Total=100, Used=0, ResetAt=zero
	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be backfilled")
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W backfilled with Total=100, got %d", tier1W.Total)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be backfilled")
	}
	if tier1M.Total != 100 {
		t.Errorf("expected 1M backfilled with Total=100, got %d", tier1M.Total)
	}
}

func TestUsedPercentCalculation(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usedPercent := 75.0

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         usedPercent,
				"limit_window_seconds": int64(18000),
				"reset_at":             int64(1745270400),
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	expectedUsed := int64(75)
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used = 100 * 75 / 100 = 75, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 (normalizing factor), got %d", tier5H.Total)
	}
}

func TestSecondaryWindowMapsTo1W(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usedPercent := 60.0
	resetAt := int64(1745270400)

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         usedPercent,
				"limit_window_seconds": int64(18000),
				"reset_at":             resetAt,
			},
			"secondary_window": map[string]interface{}{
				"used_percent":         40.0,
				"limit_window_seconds": int64(604800),
				"reset_at":             resetAt,
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be present from secondary_window")
	}

	tier1W := snapshot.Quotas[domain.Tier1W]
	expectedUsed := int64(40)
	if tier1W.Used != expectedUsed {
		t.Errorf("expected used %d, got %d", expectedUsed, tier1W.Used)
	}
}

func TestNoRateLimits(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// With backfill, all 3 canonical tiers are always present with zero values
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 backfilled tiers, got %d", len(snapshot.Quotas))
	}

	// Verify all tiers are backfilled with correct zero values
	for _, tier := range []domain.Tier{domain.Tier5H, domain.Tier1W, domain.Tier1M} {
		q, ok := snapshot.Quotas[tier]
		if !ok {
			t.Errorf("expected tier %s to be present (backfilled)", tier)
			continue
		}
		if q.Used != 0 || q.Total != 100 {
			t.Errorf("expected tier %s backfilled with Used=0, Total=100, got Used=%d, Total=%d",
				tier, q.Used, q.Total)
		}
	}
}

func TestPlanTypeAndCreditsNotLeaked(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"plan_type": "enterprise",
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         10.0,
				"limit_window_seconds": int64(18000),
				"reset_at":             int64(1745270400),
			},
		},
		"credits": map[string]interface{}{
			"has_credits": false,
			"unlimited":   false,
			"balance":     "0.00",
		},
		"rate_limit_reached_type": map[string]interface{}{
			"type": "hard_limit",
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}

	// plan_type/credits/rate_limit_reached_type do not create extra tiers beyond the actual rate limits,
	// but backfill ensures all 3 canonical tiers are present
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 tiers (5H + backfilled 1W + 1M), got %d", len(snapshot.Quotas))
	}
}

func TestCodexBackfillsMissingTiers(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/wham/usage", http.StatusOK, map[string]interface{}{
		"rate_limit": map[string]interface{}{
			"primary_window": map[string]interface{}{
				"used_percent":         30.0,
				"limit_window_seconds": int64(18000),
				"reset_at":             int64(1745270400),
			},
		},
	})

	adapter := New("codex", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify all 3 canonical tiers are present
	for _, tier := range []domain.Tier{domain.Tier5H, domain.Tier1W, domain.Tier1M} {
		_, ok := snapshot.Quotas[tier]
		if !ok {
			t.Errorf("expected tier %s to be present (backfilled)", tier)
		}
	}

	// Verify 5H has the actual values from the response
	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != 30 {
		t.Errorf("expected 5H Used=30, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H Total=100, got %d", tier5H.Total)
	}

	// Verify backfilled tiers have zero values
	for _, tier := range []domain.Tier{domain.Tier1W, domain.Tier1M} {
		q := snapshot.Quotas[tier]
		if q.Used != 0 || q.Total != 100 {
			t.Errorf("expected tier %s backfilled with Used=0, Total=100, got Used=%d, Total=%d",
				tier, q.Used, q.Total)
		}
	}
}