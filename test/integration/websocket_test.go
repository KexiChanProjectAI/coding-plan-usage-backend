package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quotahub/ucpqa/internal/app"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestWebSocketRefreshAndMetricsExposure(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19201,
			MetricsPort: 19202,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	metricsServer := httptest.NewServer(app.MetricsServer().Handler)
	defer metricsServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "ws-test-provider",
		AccountAlias: "ws-test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 15, Total: 150, ResetAt: time.Now().Add(4 * time.Hour)},
			domain.Tier1W: {Used: 25, Total: 250, ResetAt: time.Now().Add(7 * 24 * time.Hour)},
		},
		Version: 1,
	}

	st.Update(snapshot)
	m.UpdateFromSnapshot(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	wsHub.Broadcast(msgBytes)

	time.Sleep(100 * time.Millisecond)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "ws-test-provider")

	req, err := http.NewRequest("GET", metricsServer.URL+"/metrics", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, "coding_plan_usage_value")
	assert.Contains(t, content, "coding_plan_reset_timestamp")
	assert.Contains(t, content, `platform="ws-test-provider"`)
	assert.Contains(t, content, `account="ws-test-account"`)
	assert.Contains(t, content, `tier="5H"`)
	assert.Contains(t, content, `tier="1W"`)
}

func TestWebSocketReceivesBroadcastAfterStoreUpdate(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19211,
			MetricsPort: 19212,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "broadcast-provider",
		AccountAlias: "broadcast-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 5, Total: 50, ResetAt: time.Now().Add(3 * time.Hour)},
		},
		Version: 1,
	}

	st.Update(snapshot)
	msgBytes, _ := json.Marshal(snapshot)
	wsHub.Broadcast(msgBytes)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var received map[string]interface{}
	err = json.Unmarshal(msg, &received)
	require.NoError(t, err)

	assert.Equal(t, "broadcast-provider", received["platform"])
	assert.Equal(t, float64(1), received["version"])
}

func TestWebSocketBroadcastToMultipleClients(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19231,
			MetricsPort: 19232,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn1, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	testMsg := []byte(`{"platform":"broadcast-test","version":42}`)
	wsHub.Broadcast(testMsg)

	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg1, err := conn1.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, string(testMsg), string(msg1))

	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg2, err := conn2.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, string(testMsg), string(msg2))
}

func TestWebSocketNonRefreshMessageDoesNotCauseError(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19241,
			MetricsPort: 19242,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteJSON(map[string]string{"type": "unknown"})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "after-unknown-msg",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 1, Total: 10, ResetAt: time.Now().Add(1 * time.Hour)},
		},
		Version: 1,
	}
	st.Update(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	wsHub.Broadcast(msgBytes)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "after-unknown-msg")
}

func TestWebSocketClientCanSendRefreshMessage(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19261,
			MetricsPort: 19262,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteJSON(map[string]string{"type": "refresh"})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "after-refresh",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 1, Total: 10, ResetAt: time.Now().Add(1 * time.Hour)},
		},
		Version: 1,
	}
	st.Update(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	wsHub.Broadcast(msgBytes)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "after-refresh")
}

func TestWebSocketUpgradeFailsForHTTPRequest(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19301,
			MetricsPort: 19302,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	req, err := http.NewRequest("GET", apiServer.URL+"/ws", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebSocketMultipleBroadcastsArriveInOrder(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19311,
			MetricsPort: 19312,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	for i := 1; i <= 3; i++ {
		snapshot := domain.AccountSnapshot{
			Platform:     "multi-broadcast",
			AccountAlias: "test",
			Quotas: map[domain.Tier]domain.QuotaTier{
				domain.Tier5H: {Used: int64(i), Total: 100, ResetAt: time.Now().Add(1 * time.Hour)},
			},
			Version: int64(i),
		}
		st.Update(snapshot)
		time.Sleep(50 * time.Millisecond)
	}

	for i := 1; i <= 3; i++ {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)

		var received map[string]interface{}
		err = json.Unmarshal(msg, &received)
		require.NoError(t, err)

		assert.Equal(t, float64(i), received["version"])
	}
}

func TestWebSocketInvalidJSONHandledGracefully(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     19251,
			MetricsPort: 19252,
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

	apiServer := httptest.NewServer(app.APIServer().Handler)
	defer apiServer.Close()

	wsHub := app.WSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	u := "ws" + strings.TrimPrefix(apiServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteMessage(websocket.TextMessage, []byte("not valid json{{{"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	snapshot := domain.AccountSnapshot{
		Platform:     "after-invalid-json",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 1, Total: 10, ResetAt: time.Now().Add(1 * time.Hour)},
		},
		Version: 1,
	}
	st.Update(snapshot)

	msgBytes, _ := json.Marshal(snapshot)
	wsHub.Broadcast(msgBytes)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "after-invalid-json")
}
