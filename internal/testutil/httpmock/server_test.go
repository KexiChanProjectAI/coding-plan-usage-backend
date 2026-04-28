package httpmock

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockServer(t *testing.T) {
	t.Run("New creates server with valid URL", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		assert.NotEmpty(t, ms.URL())
		assert.Contains(t, ms.URL(), "http://")
	})

	t.Run("SetResponse configures JSON response", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		data := map[string]string{"key": "value"}
		err := ms.SetResponse("/test", http.StatusOK, data)
		require.NoError(t, err)

		resp, err := http.Get(ms.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["key"])
	})

	t.Run("SetResponse handles string body", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		err := ms.SetResponse("/text", http.StatusOK, "plain text response")
		require.NoError(t, err)

		resp, err := http.Get(ms.URL() + "/text")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "plain text response", string(body))
	})

	t.Run("SetResponse handles byte slice body", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		err := ms.SetResponse("/bytes", http.StatusCreated, []byte(`{"raw":"bytes"}`))
		require.NoError(t, err)

		resp, err := http.Get(ms.URL() + "/bytes")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("SetResponse returns error for non-marshallable body", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		complex := make(chan int)
		err := ms.SetResponse("/bad", http.StatusOK, complex)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal body")
	})

	t.Run("SetResponseWithDelay adds delay before response", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		err := ms.SetResponseWithDelay("/delayed", http.StatusOK, "delayed", 50*time.Millisecond)
		require.NoError(t, err)

		start := time.Now()
		resp, err := http.Get(ms.URL() + "/delayed")
		elapsed := time.Since(start)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	})

	t.Run("unconfigured path returns 404", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		resp, err := http.Get(ms.URL() + "/nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("URL returns correct base URL", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		url := ms.URL()
		assert.NotEmpty(t, url)
		assert.True(t, len(url) > len("http://"))
	})

	t.Run("Close terminates server", func(t *testing.T) {
		ms := New()
		ms.Close()

		resp, err := http.Get(ms.URL() + "/test")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("multiple paths with different responses", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		require.NoError(t, ms.SetResponse("/ok", http.StatusOK, "OK"))
		require.NoError(t, ms.SetResponse("/created", http.StatusCreated, "Created"))
		require.NoError(t, ms.SetResponse("/error", http.StatusInternalServerError, "Error"))

		resp1, err := http.Get(ms.URL() + "/ok")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		resp1.Body.Close()

		resp2, err := http.Get(ms.URL() + "/created")
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp2.StatusCode)
		resp2.Body.Close()

		resp3, err := http.Get(ms.URL() + "/error")
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
		resp3.Body.Close()
	})
}

func TestHTTPMockServer(t *testing.T) {
	t.Run(" httptest.Server integration", func(t *testing.T) {
		ms := New()
		defer ms.Close()

		serverURL := ms.URL()

		req := httptest.NewRequest(http.MethodGet, serverURL+"/health", nil)
		w := httptest.NewRecorder()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
