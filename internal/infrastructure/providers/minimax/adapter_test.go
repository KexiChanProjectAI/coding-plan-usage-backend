package minimax

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestFetchQuotaSuccess(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"start_time":                  1745164800000,
				"end_time":                    1745186400000,
				"remains_time":                21600000,
				"current_interval_total_count": 100,
				"current_interval_usage_count": 30,
				"model_name":                  "coding-plan-MiniMax-M2",
				"current_weekly_total_count":   500,
				"current_weekly_usage_count":   150,
				"weekly_start_time":            1744560000000,
				"weekly_end_time":              1745164800000,
				"weekly_remains_time":          604800000,
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "minimax" {
		t.Errorf("expected platform minimax, got %s", snapshot.Platform)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Used != 30 {
		t.Errorf("expected used 30, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present")
	}
	if tier1W.Used != 30 {
		t.Errorf("expected used 30, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected total 100, got %d", tier1W.Total)
	}

	tier1M, ok := snapshot.Quotas[domain.Tier1M]
	if !ok {
		t.Fatal("expected 1M tier to be backfilled")
	}
	if tier1M.Used != 0 {
		t.Errorf("expected 1M used 0, got %d", tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("expected 1M total 100, got %d", tier1M.Total)
	}
}

func TestCurrentIntervalUsageCountMeansUsed(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 100,
				"current_interval_usage_count": 25,
				"current_weekly_total_count":   0,
				"current_weekly_usage_count":   0,
				"end_time":                     1745186400000,
				"model_name":                   "coding-plan-test",
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}

	expectedUsed := int64(25)
	if tier5H.Used != expectedUsed {
		t.Errorf("expected used = 25, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected total 100, got %d", tier5H.Total)
	}
}

func TestWeeklyQuotaOmittedWhenZero(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 100,
				"current_interval_usage_count": 50,
				"current_weekly_total_count":   500,
				"current_weekly_usage_count":   0,
				"end_time":                     1745186400000,
				"weekly_end_time":              1745164800000,
				"model_name":                   "coding-plan-test",
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}

	tier1W, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be present when current_weekly_total_count > 0")
	}
	if tier1W.Used != 0 {
		t.Errorf("expected 1W used 0, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total 100, got %d", tier1W.Total)
	}
}

func TestUpstreamRejection(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"base_resp": map[string]interface{}{
			"status_code": 1001,
			"status_msg":  "invalid token",
		},
	})

	adapter := New("minimax", server.URL(), "bad-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for upstream rejection")
	}

	if !provider.IsUpstreamRejection(err) {
		t.Errorf("expected upstream rejection error, got %T", err)
	}

	upstreamErr := err.(*provider.ErrUpstreamRejection)
	if upstreamErr.StatusCode != 1001 {
		t.Errorf("expected status code 1001, got %d", upstreamErr.StatusCode)
	}
	if upstreamErr.Message != "invalid token" {
		t.Errorf("expected message 'invalid token', got %s", upstreamErr.Message)
	}
}

func TestHTTPError(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusInternalServerError, nil)

	adapter := New("minimax", server.URL(), "test-token")
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

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, "invalid json{")

	adapter := New("minimax", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error for parse failure")
	}

	if !provider.IsParseFailure(err) {
		t.Errorf("expected parse failure error, got %T", err)
	}
}

func TestProviderName(t *testing.T) {
	adapter := New("minimax", "https://example.com", "token")
	if adapter.ProviderName() != "minimax" {
		t.Errorf("expected provider name 'minimax', got %s", adapter.ProviderName())
	}
}

func TestNoModelRemains(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have backfilled tiers even with no models
	tier5H, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be backfilled")
	}
	if tier5H.Used != 0 {
		t.Errorf("expected 5H used 0, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total 100, got %d", tier5H.Total)
	}

	tier1W, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be backfilled")
	}
	if tier1W.Used != 0 {
		t.Errorf("expected 1W used 0, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total 100, got %d", tier1W.Total)
	}

	tier1M, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be backfilled")
	}
	if tier1M.Used != 0 {
		t.Errorf("expected 1M used 0, got %d", tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("expected 1M total 100, got %d", tier1M.Total)
	}
}

func TestMultipleModelRemains(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 100,
				"current_interval_usage_count": 20,
				"current_weekly_total_count":   0,
				"current_weekly_usage_count":   0,
				"end_time":                     1745186400000,
				"model_name":                   "coding-plan-model1",
			},
			{
				"current_interval_total_count": 200,
				"current_interval_usage_count": 100,
				"current_weekly_total_count":   1000,
				"current_weekly_usage_count":   300,
				"end_time":                     1745186400000,
				"weekly_end_time":              1745164800000,
				"model_name":                   "coding-plan-model2",
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be present")
	}
	// Highest 5H percent: model1 (20/100) = 20%, model2 (100/200) = 50%
	if tier5H.Used != 50 {
		t.Errorf("expected 5H used 50 (highest of 20%% and 50%%), got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total 100, got %d", tier5H.Total)
	}

	tier1W, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be present since at least one model has weekly quota")
	}
	// Highest 1W percent: model2 (300/1000) = 30%
	if tier1W.Used != 30 {
		t.Errorf("expected 1W used 30, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total 100, got %d", tier1W.Total)
	}

	// Check backfilled 1M tier
	tier1M, has1M := snapshot.Quotas[domain.Tier1M]
	if !has1M {
		t.Fatal("expected 1M tier to be backfilled")
	}
	if tier1M.Used != 0 {
		t.Errorf("expected 1M used 0, got %d", tier1M.Used)
	}
	if tier1M.Total != 100 {
		t.Errorf("expected 1M total 100, got %d", tier1M.Total)
	}
}

func TestNonCodingPlanModelsIgnored(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 100,
				"current_interval_usage_count": 50,
				"current_weekly_total_count":   500,
				"current_weekly_usage_count":   250,
				"end_time":                     1745186400000,
				"weekly_end_time":              1745164800000,
				"model_name":                   "MiniMax-M2", // No coding-plan prefix
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have backfilled tiers with 0% since no coding-plan models
	tier5H, has5H := snapshot.Quotas[domain.Tier5H]
	if !has5H {
		t.Fatal("expected 5H tier to be backfilled")
	}
	if tier5H.Used != 0 {
		t.Errorf("expected 5H used 0, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total 100, got %d", tier5H.Total)
	}

	tier1W, has1W := snapshot.Quotas[domain.Tier1W]
	if !has1W {
		t.Fatal("expected 1W tier to be backfilled")
	}
	if tier1W.Used != 0 {
		t.Errorf("expected 1W used 0, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total 100, got %d", tier1W.Total)
	}
}

func TestResetAtSetForValidTiers(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	endTime := int64(1745186400000) // 2025-04-28 10:00:00 UTC
	weeklyEndTime := int64(1745164800000) // 2025-04-28 04:00:00 UTC

	server.SetResponse("/v1/api/openplatform/coding_plan/remains", http.StatusOK, map[string]interface{}{
		"model_remains": []map[string]interface{}{
			{
				"current_interval_total_count": 100,
				"current_interval_usage_count": 30,
				"current_weekly_total_count":   500,
				"current_weekly_usage_count":   150,
				"end_time":                     endTime,
				"weekly_end_time":              weeklyEndTime,
				"model_name":                   "coding-plan-test",
			},
		},
		"base_resp": map[string]interface{}{
			"status_code": 0,
			"status_msg":  "success",
		},
	})

	adapter := New("minimax", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Used != 30 {
		t.Errorf("expected 5H used 30, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Errorf("expected 5H total 100, got %d", tier5H.Total)
	}
	if tier5H.ResetAt.IsZero() {
		t.Error("expected 5H reset_at to be set")
	}
	if !tier5H.ResetAt.Equal(time.UnixMilli(endTime)) {
		t.Errorf("expected 5H reset_at %v, got %v", time.UnixMilli(endTime), tier5H.ResetAt)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present")
	}
	if tier1W.Used != 30 {
		t.Errorf("expected 1W used 30, got %d", tier1W.Used)
	}
	if tier1W.Total != 100 {
		t.Errorf("expected 1W total 100, got %d", tier1W.Total)
	}
	if tier1W.ResetAt.IsZero() {
		t.Error("expected 1W reset_at to be set")
	}
	if !tier1W.ResetAt.Equal(time.UnixMilli(weeklyEndTime)) {
		t.Errorf("expected 1W reset_at %v, got %v", time.UnixMilli(weeklyEndTime), tier1W.ResetAt)
	}
}
