// Package httpmock provides a lightweight HTTP mock server for testing.
package httpmock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// Response defines a configured response for a given path.
type Response struct {
	StatusCode int
	Body       []byte
	Delay      time.Duration
}

// MockServer is a test helper that simulates an HTTP server with configurable responses.
type MockServer struct {
	server *httptest.Server
	mux    *http.ServeMux

	mu       sync.RWMutex
	responses map[string]Response
}

// New creates a new MockServer with a default handler.
func New() *MockServer {
	ms := &MockServer{
		responses: make(map[string]Response),
	}
	ms.mux = http.NewServeMux()
	ms.mux.HandleFunc("/", ms.handle)
	ms.server = httptest.NewServer(ms.mux)
	return ms
}

// handle routes incoming requests to configured responses.
func (ms *MockServer) handle(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	resp, ok := ms.responses[r.URL.Path]
	ms.mu.RUnlock()

	if !ok {
		http.Error(w, "no response configured for path: "+r.URL.Path, http.StatusNotFound)
		return
	}

	if resp.Delay > 0 {
		time.Sleep(resp.Delay)
	}

	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body) //nolint:errcheck
	}
}

// SetResponse configures a response for a given path with status code and body.
func (ms *MockServer) SetResponse(path string, statusCode int, body interface{}) error {
	var bodyBytes []byte
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyBytes = []byte(v)
		case []byte:
			bodyBytes = v
		default:
			var err error
			bodyBytes, err = json.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
		}
	}

	ms.mu.Lock()
	ms.responses[path] = Response{
		StatusCode: statusCode,
		Body:       bodyBytes,
	}
	ms.mu.Unlock()

	return nil
}

// SetResponseWithDelay configures a response for a given path with status code, body, and delay.
func (ms *MockServer) SetResponseWithDelay(path string, statusCode int, body interface{}, delay time.Duration) error {
	var bodyBytes []byte
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyBytes = []byte(v)
		case []byte:
			bodyBytes = v
		default:
			var err error
			bodyBytes, err = json.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
		}
	}

	ms.mu.Lock()
	ms.responses[path] = Response{
		StatusCode: statusCode,
		Body:       bodyBytes,
		Delay:      delay,
	}
	ms.mu.Unlock()

	return nil
}

// URL returns the base URL of the mock server.
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// Close shuts down the mock server.
func (ms *MockServer) Close() {
	ms.server.Close()
}

// HTTPClient returns an http.Client that can be used to make requests to the mock server.
func (ms *MockServer) HTTPClient() *http.Client {
	return ms.server.Client()
}
