package monitorquota

import (
	"context"
	"net/http"
	"testing"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestFetchQuotaSuccessZAI(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usage := int64(30)
	currentValue := int64(100)
	remaining := int64(70)
	percentage := 30
	nextResetTime := int64(1745186400000)

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code": 200,
		"msg":  "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TOKENS_LIMIT",
					"unit":          3,
					"number":        5,
					"usage":         usage,
					"currentValue":  currentValue,
					"remaining":     remaining,
					"percentage":    percentage,
					"nextResetTime": nextResetTime,
					"usageDetails":  []map[string]interface{}{},
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "zai" {
		t.Errorf("expected platform zai, got %s", snapshot.Platform)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Used != int64(percentage) {
		t.Errorf("expected used %d from percentage field, got %d", percentage, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 when percentage is present, got %d", tier5H.Total)
	}
}

func TestFetchQuotaSuccessZhipu(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usage := int64(50)
	currentValue := int64(200)
	percentage := 25
	nextResetTime := int64(1745186400000)

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code": 200,
		"msg":  "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TIME_LIMIT",
					"unit":          5,
					"number":        1,
					"usage":         usage,
					"currentValue":  currentValue,
					"remaining":     150,
					"percentage":    25,
					"nextResetTime": nextResetTime,
					"usageDetails":  []map[string]interface{}{},
				},
			},
			"level": "premium",
		},
	})

	adapter := NewZhipu("zhipu", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "zhipu" {
		t.Errorf("expected platform zhipu, got %s", snapshot.Platform)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be present")
	}
	if tier1M.Used != int64(percentage) {
		t.Errorf("expected used %d from percentage field, got %d", percentage, tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("expected total 100 when percentage is present, got %d", tier1M.Total)
	}
}

func TestRejectsFailedEnvelope(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    400,
		"msg":     "invalid request",
		"success": false,
		"data":    nil,
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for failed envelope")
	}

	if !provider.IsUpstreamRejection(err) {
		t.Errorf("expected upstream rejection error, got %T", err)
	}

	upstreamErr := err.(*provider.ErrUpstreamRejection)
	if upstreamErr.StatusCode != 400 {
		t.Errorf("expected status code 400, got %d", upstreamErr.StatusCode)
	}
	if upstreamErr.Message != "invalid request" {
		t.Errorf("expected message 'invalid request', got %s", upstreamErr.Message)
	}
}

func TestRejectsSuccessFalse(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "ok",
		"success": false,
		"data":    nil,
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error when success is false")
	}

	if !provider.IsUpstreamRejection(err) {
		t.Errorf("expected upstream rejection error, got %T", err)
	}
}

func TestUnitMapping(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "HOURLY_TOKEN",
					"unit":          3,
					"usage":         10,
					"currentValue":  50,
					"nextResetTime": 1745186400000,
				},
				{
					"type":          "MONTHLY_TOKEN",
					"unit":          5,
					"usage":         100,
					"currentValue":  500,
					"nextResetTime": 1747780800000,
				},
				{
					"type":          "WEEKLY_TOKEN",
					"unit":          4,
					"usage":         25,
					"currentValue":  100,
					"nextResetTime": 1745270400000,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present for unit=3")
	}

	// Verify 5H uses normalized percentage (usage/currentValue * 100 = 10/50*100 = 20)
	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total 100, got %d", tier5H.Total)
	}
	if tier5H.Used != domain.NormalizeToPercent(10, 50) {
		t.Errorf("expected 5H used %d from usage/currentValue, got %d", domain.NormalizeToPercent(10, 50), tier5H.Used)
	}

	_, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be present for unit=5")
	}

	// Verify 1M uses normalized percentage (usage/currentValue * 100 = 100/500*100 = 20)
	tier1M := snapshot.Quotas[domain.Tier1M]
	if tier1M.Total != 100 {
		t.Errorf("expected 1M total 100, got %d", tier1M.Total)
	}
	if tier1M.Used != domain.NormalizeToPercent(100, 500) {
		t.Errorf("expected 1M used %d from usage/currentValue, got %d", domain.NormalizeToPercent(100, 500), tier1M.Used)
	}

	_, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be backfilled since it's a canonical tier")
	}
	// 1W should be backfilled with defaults since unit=4 is unsupported
	tier1W := snapshot.Quotas[domain.Tier1W]
	if tier1W.Used != 0 {
		t.Errorf("expected 1W used=0 (backfilled), got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total=100 (backfilled), got %d", tier1W.Total)
	}
	if !tier1W.ResetAt.IsZero() {
		t.Errorf("expected 1W resetAt=zero (backfilled), got %v", tier1W.ResetAt)
	}
}

func TestUsedCalculationFromUsageField(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usage := int64(45)
	currentValue := int64(100)

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TOKEN_LIMIT",
					"unit":          3,
					"usage":         usage,
					"currentValue":  currentValue,
					"nextResetTime": 1745186400000,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H := snapshot.Quotas[domain.Tier5H]
	expectedUsed := domain.NormalizeToPercent(usage, currentValue)
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used = NormalizeToPercent(%d, %d) = %d, got %d", usage, currentValue, expectedUsed, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}
}

func TestUsedCalculationFromCurrentValueMinusRemaining(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	currentValue := int64(100)
	remaining := int64(65)

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TOKEN_LIMIT",
					"unit":          3,
					"currentValue":  currentValue,
					"remaining":     remaining,
					"nextResetTime": 1745186400000,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H := snapshot.Quotas[domain.Tier5H]
	expectedUsed := domain.NormalizeToPercent(currentValue-remaining, currentValue)
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used = NormalizeToPercent(%d, %d) = %d, got %d", currentValue-remaining, currentValue, expectedUsed, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}
}

func TestProviderNameZAI(t *testing.T) {
	adapter := NewZAI("zai", "https://example.com", "token")
	if adapter.ProviderName() != "zai" {
		t.Errorf("expected provider name 'zai', got %s", adapter.ProviderName())
	}
}

func TestProviderNameZhipu(t *testing.T) {
	adapter := NewZhipu("zhipu", "https://example.com", "token")
	if adapter.ProviderName() != "zhipu" {
		t.Errorf("expected provider name 'zhipu', got %s", adapter.ProviderName())
	}
}

func TestHTTPError(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusInternalServerError, nil)

	adapter := NewZAI("zai", server.URL(), "test-token")
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

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, "invalid json{")

	adapter := NewZAI("zai", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for parse failure")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestEmptyLimits(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{},
			"level":  "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// All three canonical tiers should be backfilled
	if len(snapshot.Quotas) != 3 {
		t.Errorf("expected 3 backfilled tiers (5H, 1W, 1M), got %d tiers", len(snapshot.Quotas))
	}

	// All three canonical tiers should be backfilled with Used=0, Total=100, ResetAt=zero
	for _, tier := range []domain.Tier{domain.Tier5H, domain.Tier1W, domain.Tier1M} {
		qt, ok := snapshot.Quotas[tier]
		if !ok {
			t.Errorf("expected backfilled tier %s to be present", tier)
			continue
		}
		if qt.Used != 0 {
			t.Errorf("expected tier %s used=0, got %d", tier, qt.Used)
		}
		if qt.Total != 100 {
			t.Errorf("expected tier %s total=100, got %d", tier, qt.Total)
		}
		if !qt.ResetAt.IsZero() {
			t.Errorf("expected tier %s resetAt=zero, got %v", tier, qt.ResetAt)
		}
	}
}

func TestMultipleTiersZAI(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "HOURLY_TOKEN",
					"unit":          3,
					"usage":         10,
					"currentValue":  50,
					"nextResetTime": 1745186400000,
				},
				{
					"type":          "MONTHLY_TOKEN",
					"unit":          5,
					"usage":         100,
					"currentValue":  500,
					"nextResetTime": 1747780800000,
				},
			},
			"level": "premium",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}
	if snapshot.Quotas[domain.Tier5H].Total != 100 {
		t.Errorf("expected 5H total 100, got %d", snapshot.Quotas[domain.Tier5H].Total)
	}

	_, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be present")
	}
	if snapshot.Quotas[domain.Tier1M].Total != 100 {
		t.Errorf("expected 1M total 100, got %d", snapshot.Quotas[domain.Tier1M].Total)
	}
}

func TestPercentageTakesPrecedence(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	currentValue := int64(100)
	remaining := int64(60)
	percentage := 40

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TOKEN_LIMIT",
					"unit":          3,
					"currentValue":  currentValue,
					"remaining":     remaining,
					"percentage":    percentage,
					"nextResetTime": 1745186400000,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != int64(percentage) {
		t.Errorf("expected used to be from percentage (%d), not computed (%d)", percentage, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100 when percentage is present, got %d", tier5H.Total)
	}
}

func TestPercentagePrecedenceWithNilCurrentValue(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	percentage := 75

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":       "TOKEN_LIMIT",
					"unit":       3,
					"percentage": percentage,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != int64(percentage) {
		t.Errorf("expected used from percentage (%d), got %d", percentage, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}
}

func TestPercentagePrecedenceWithZeroCurrentValue(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	percentage := 50

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":         "TOKEN_LIMIT",
					"unit":         3,
					"currentValue": 0,
					"percentage":   percentage,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != int64(percentage) {
		t.Errorf("expected used from percentage (%d), got %d", percentage, tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}
}

func TestZeroDenominatorSkipsTier(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":         "TOKEN_LIMIT",
					"unit":         3,
					"currentValue": 0,
					"remaining":    0,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Tier is skipped during extraction (currentValue=0, no percentage)
	// but backfilled as a canonical tier with defaults
	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be backfilled as canonical tier")
	}
	// Verify 5H has backfilled defaults, not provider values
	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != 0 {
		t.Errorf("expected 5H used=0 (backfilled), got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total=100 (backfilled), got %d", tier5H.Total)
	}
	if !tier5H.ResetAt.IsZero() {
		t.Errorf("expected 5H resetAt=zero (backfilled), got %v", tier5H.ResetAt)
	}
}

func TestMonitorQuotaBackfillsMissingTiers(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/api/monitor/usage/quota/limit", http.StatusOK, map[string]interface{}{
		"code":    200,
		"msg":     "success",
		"success": true,
		"data": map[string]interface{}{
			"limits": []map[string]interface{}{
				{
					"type":          "TOKEN_LIMIT",
					"unit":          3,
					"percentage":    25,
					"nextResetTime": 1745186400000,
				},
			},
			"level": "standard",
		},
	})

	adapter := NewZAI("zai", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify all three canonical tiers are present
	for _, tier := range []domain.Tier{domain.Tier5H, domain.Tier1W, domain.Tier1M} {
		if _, ok := snapshot.Quotas[tier]; !ok {
			t.Errorf("expected canonical tier %s to be backfilled", tier)
		}
	}

	// Verify 5H has actual data
	tier5H := snapshot.Quotas[domain.Tier5H]
	if tier5H.Used != 25 {
		t.Errorf("expected 5H used=25, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total=100, got %d", tier5H.Total)
	}

	// Verify 1W is backfilled with defaults
	tier1W := snapshot.Quotas[domain.Tier1W]
	if tier1W.Used != 0 {
		t.Errorf("expected 1W used=0, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total=100, got %d", tier1W.Total)
	}
	if !tier1W.ResetAt.IsZero() {
		t.Errorf("expected 1W resetAt=zero, got %v", tier1W.ResetAt)
	}

	// Verify 1M is backfilled with defaults
	tier1M := snapshot.Quotas[domain.Tier1M]
	if tier1M.Used != 0 {
		t.Errorf("expected 1M used=0, got %d", tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("expected 1M total=100, got %d", tier1M.Total)
	}
	if !tier1M.ResetAt.IsZero() {
		t.Errorf("expected 1M resetAt=zero, got %v", tier1M.ResetAt)
	}
}