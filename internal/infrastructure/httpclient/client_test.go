package httpclient

import (
	"net/http"
	"strings"
	"testing"

	"github.com/quotahub/ucpqa/internal/testutil/httpmock"
)

func TestNewClient(t *testing.T) {
	client := NewClient(DefaultConnectTimeout)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.Timeout != DefaultConnectTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultConnectTimeout, client.Timeout)
	}
}

func TestNewClientWithDefaults(t *testing.T) {
	client := NewClientWithDefaults()
	if client == nil {
		t.Fatal("NewClientWithDefaults returned nil")
	}
	if client.Timeout != DefaultConnectTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultConnectTimeout, client.Timeout)
	}
}

func TestSetAuthHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	SetAuthHeader(req, "test-token")

	expected := "Bearer test-token"
	if got := req.Header.Get("Authorization"); got != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, got)
	}
}

func TestSetAuthHeader_MultipleCalls(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	SetAuthHeader(req, "token1")
	SetAuthHeader(req, "token2")

	expected := "Bearer token2"
	if got := req.Header.Get("Authorization"); got != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, got)
	}
}

func TestDecodeJSON(t *testing.T) {
	jsonData := `{"name":"test","value":42}`
	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := DecodeJSON(strings.NewReader(jsonData), &result)
	if err != nil {
		t.Fatalf("DecodeJSON failed: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected name 'test', got %q", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("expected value 42, got %d", result.Value)
	}
}

func TestDecodeJSON_IgnoreUnknownKeys(t *testing.T) {
	jsonData := `{"name":"test","unknown_field":"should be ignored","another":123}`
	var result struct {
		Name string `json:"name"`
	}

	err := DecodeJSON(strings.NewReader(jsonData), &result)
	if err != nil {
		t.Fatalf("DecodeJSON failed: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected name 'test', got %q", result.Name)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json}`
	var result struct{}

	err := DecodeJSON(strings.NewReader(invalidJSON), &result)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestDecodeJSON_EmptyInput(t *testing.T) {
	var result struct{}

	err := DecodeJSON(strings.NewReader(""), &result)
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestClient_IntegrationWithMockServer(t *testing.T) {
	ms := httpmock.New()
	defer ms.Close()

	if err := ms.SetResponse("/test", http.StatusOK, `{"status":"ok"}`); err != nil {
		t.Fatalf("failed to set response: %v", err)
	}

	client := NewClientWithDefaults()
	req, err := http.NewRequest(http.MethodGet, ms.URL()+"/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	SetAuthHeader(req, "test-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := DecodeJSON(resp.Body, &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", result.Status)
	}
}