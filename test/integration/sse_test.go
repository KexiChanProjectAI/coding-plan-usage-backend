package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quotahub/ucpqa/internal/app"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/quotahub/ucpqa/internal/transport/sse"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSSEReceivesSnapshotDiff(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19101,
			MetricsPort: 19102,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	sseBroker := app.SSEBroker()
	go sseBroker.Run()
	defer sseBroker.Stop()

	handler := sse.NewHandler(sseBroker)

	router := gin.New()
	router.GET("/api/v1/stream", handler.StreamHandler())

	sseServer := httptest.NewServer(router)
	defer sseServer.Close()

	req, err := http.NewRequest("GET", sseServer.URL+"/api/v1/stream", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	time.Sleep(50 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "test-provider",
		AccountAlias: "test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
		},
		Version: 1,
	}

	st.Update(snapshot)
	m.UpdateFromSnapshot(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	sseBroker.Publish(msgBytes)

	reader := io.LimitReader(resp.Body, 4096)
	buf := make([]byte, 4096)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		n, _ := reader.Read(buf)
		if n > 0 {
			close(done)
		}
	}()

	select {
	case <-done:
		msg := string(buf[:])
		assert.True(t, strings.HasPrefix(strings.TrimSpace(msg), "data: "),
			"expected SSE message to start with 'data: ', got: %s", msg)
		assert.Contains(t, msg, "test-provider")
		assert.Contains(t, msg, "test-account")
	case <-ctx.Done():
		t.Fatal("timeout waiting for SSE message within 5 seconds")
	}
}

func TestSSEMultipleClientsReceiveBroadcast(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19111,
			MetricsPort: 19112,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	sseBroker := app.SSEBroker()
	go sseBroker.Run()
	defer sseBroker.Stop()

	handler := sse.NewHandler(sseBroker)

	router := gin.New()
	router.GET("/api/v1/stream", handler.StreamHandler())

	sseServer := httptest.NewServer(router)
	defer sseServer.Close()

	req1, _ := http.NewRequest("GET", sseServer.URL+"/api/v1/stream", nil)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()

	req2, _ := http.NewRequest("GET", sseServer.URL+"/api/v1/stream", nil)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	time.Sleep(50 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "broadcast-test",
		AccountAlias: "test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 5, Total: 50, ResetAt: time.Now().Add(3 * time.Hour)},
		},
		Version: 1,
	}
	st.Update(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	sseBroker.Publish(msgBytes)

	time.Sleep(100 * time.Millisecond)

	buf1 := make([]byte, 1024)
	n1, _ := resp1.Body.Read(buf1)
	msg1 := string(buf1[:n1])
	assert.Contains(t, msg1, "data: ")
	assert.Contains(t, msg1, "broadcast-test")
}

func TestSSEEmptyStoreDoesNotPanic(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19131,
			MetricsPort: 19132,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &app.Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	require.NoError(t, err)

	sseBroker := app.SSEBroker()
	go sseBroker.Run()
	defer sseBroker.Stop()

	handler := sse.NewHandler(sseBroker)

	router := gin.New()
	router.GET("/api/v1/stream", handler.StreamHandler())

	sseServer := httptest.NewServer(router)
	defer sseServer.Close()

	req, err := http.NewRequest("GET", sseServer.URL+"/api/v1/stream", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	time.Sleep(50 * time.Millisecond)

	sseBroker.Publish([]byte(`{"platform":"empty-test","version":0}`))

	time.Sleep(100 * time.Millisecond)
}
