package sub2api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestFetchQuotaSuccessUsesMostConstrainedModelPerWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/model-quotas" {
			t.Fatalf("expected /v1/model-quotas, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"model-a","object":"model_quota","quota_pool":"account_shared","five_hour":{"total_usd":20,"used_usd":5,"remaining_usd":15,"accounts_count":1,"unknown_accounts_count":0},"seven_day":{"total_usd":100,"used_usd":30,"remaining_usd":70,"accounts_count":1,"unknown_accounts_count":0}},{"id":"model-b","object":"model_quota","quota_pool":"account_shared","five_hour":{"total_usd":20,"used_usd":10,"remaining_usd":10,"accounts_count":1,"unknown_accounts_count":0},"seven_day":{"total_usd":100,"used_usd":40,"remaining_usd":60,"accounts_count":1,"unknown_accounts_count":0}}]}`))
	}))
	defer server.Close()

	adapter := New("sub2api", server.URL, "test-token")
	snapshot, err := adapter.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Platform != "sub2api" {
		t.Fatalf("expected platform sub2api, got %s", snapshot.Platform)
	}

	tier5H, ok := snapshot.Quotas[domain.Tier5H]
	if !ok {
		t.Fatal("expected 5H tier to be present")
	}
	if tier5H.Used != 50 {
		t.Fatalf("expected highest 5H usage 50, got %d", tier5H.Used)
	}
	if tier5H.Total != 100 {
		t.Fatalf("expected 5H total 100, got %d", tier5H.Total)
	}

	tier1W, ok := snapshot.Quotas[domain.Tier1W]
	if !ok {
		t.Fatal("expected 1W tier to be present")
	}
	if tier1W.Used != 40 {
		t.Fatalf("expected highest 1W usage 40, got %d", tier1W.Used)
	}

	if _, ok := snapshot.Quotas[domain.Tier1M]; !ok {
		t.Fatal("expected 1M tier to be backfilled")
	}
}

func TestFetchQuotaSkipsWindowsWithoutTotals(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/model-quotas", http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"id":         "model-a",
			"object":     "model_quota",
			"quota_pool": "account_shared",
			"five_hour": map[string]any{
				"total_usd":     nil,
				"remaining_usd": nil,
			},
		}},
	})

	adapter := New("sub2api", server.URL(), "test-token")
	snapshot, err := adapter.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := snapshot.Quotas[domain.Tier5H].Used; got != 0 {
		t.Fatalf("expected backfilled 5H usage 0, got %d", got)
	}
	if got := snapshot.Quotas[domain.Tier1W].Used; got != 0 {
		t.Fatalf("expected backfilled 1W usage 0, got %d", got)
	}
}

func TestFetchQuotaUpstreamErrorMessage(t *testing.T) {
	server := httpmock.New()
	defer server.Close()

	server.SetResponse("/v1/model-quotas", http.StatusUnauthorized, map[string]any{
		"error": map[string]any{
			"type":    "invalid_request_error",
			"code":    "invalid_api_key",
			"message": "API key invalid",
		},
	})

	adapter := New("sub2api", server.URL(), "test-token")
	_, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !provider.IsUpstreamRejection(err) {
		t.Fatalf("expected upstream rejection, got %T", err)
	}
	if err.Error() != `upstream rejection for provider "sub2api" (status 401): API key invalid` {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestFetchQuotaInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":`))
	}))
	defer server.Close()

	adapter := New("sub2api", server.URL, "test-token")
	_, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !provider.IsParseFailure(err) {
		t.Fatalf("expected parse failure, got %T", err)
	}
}
