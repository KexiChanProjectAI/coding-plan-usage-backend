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
	if tier5H.Used != usage {
		t.Errorf("expected used %d, got %d", usage, tier5H.Used)
	}
	if tier5H.Total != currentValue {
		t.Errorf("expected total %d, got %d", currentValue, tier5H.Total)
	}
}

func TestFetchQuotaSuccessZhipu(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	usage := int64(50)
	currentValue := int64(200)
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
	if tier1M.Used != usage {
		t.Errorf("expected used %d, got %d", usage, tier1M.Used)
	}
	if tier1M.Total != currentValue {
		t.Errorf("expected total %d, got %d", currentValue, tier1M.Total)
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

	_, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be present for unit=5")
	}

	_, has1W := snapshot.Quotas[domain.Tier1W]
	if has1W {
		t.Error("expected 1W tier to be omitted for unit=4 (unsupported)")
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
	if tier5H.Used != usage {
		t.Errorf("expected used to be taken from usage field: %d, got %d", usage, tier5H.Used)
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
	expectedUsed := currentValue - remaining
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used = currentValue - remaining = %d, got %d", expectedUsed, tier5H.Used)
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

	if len(snapshot.Quotas) != 0 {
		t.Errorf("expected empty quotas, got %d tiers", len(snapshot.Quotas))
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

	_, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be present")
	}
}

func TestPercentageIgnored(t *testing.T) {
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
	computedUsed := currentValue - remaining
	if tier5H.Used != computedUsed {
		t.Errorf("expected used to be computed from currentValue-remaining (%d), not from percentage (%d)", computedUsed, int64(percentage))
	}
}