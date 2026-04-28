// Package httpclient provides shared HTTP client utilities for providers.
package httpclient

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Default timeouts (matching API documentation: 30s connect/read/write)
const (
	DefaultConnectTimeout = 30 * time.Second
	DefaultReadTimeout    = 30 * time.Second
	DefaultWriteTimeout   = 30 * time.Second
)

// NewClient creates an http.Client with configured timeouts.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// NewClientWithDefaults creates an http.Client with the default 30s timeouts.
func NewClientWithDefaults() *http.Client {
	return NewClient(DefaultConnectTimeout)
}

// SetAuthHeader sets the Authorization header to Bearer token.
func SetAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

// DecodeJSON reads JSON from an io.Reader and unmarshals it into v.
// Go's json.Unmarshal ignores unknown keys by default.
func DecodeJSON(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}