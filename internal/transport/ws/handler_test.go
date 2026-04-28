package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandler_WSHandler(t *testing.T) {
	hub := NewHub(func() {})

	handler := NewHandler(hub, Upgrader)

	router := gin.New()
	router.GET("/ws", handler.WSHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	go hub.Run()
	defer hub.Stop()

	u := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	assert.GreaterOrEqual(t, 1, clientCount, "client should be registered")
}

func TestHandler_RefreshProtocol(t *testing.T) {
	var mu sync.Mutex
	refreshCallCount := 0

	hub := NewHub(func() {
		mu.Lock()
		refreshCallCount++
		mu.Unlock()
	})

	handler := NewHandler(hub, Upgrader)

	router := gin.New()
	router.GET("/ws", handler.WSHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	go hub.Run()
	defer hub.Stop()

	u := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteJSON(map[string]string{"type": "refresh"})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, refreshCallCount)
	mu.Unlock()
}

func TestHandler_BroadcastOnUpdate(t *testing.T) {
	hub := NewHub(func() {})

	handler := NewHandler(hub, Upgrader)

	router := gin.New()
	router.GET("/ws", handler.WSHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	go hub.Run()
	defer hub.Stop()

	u := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	broadcastMsg := []byte(`{"platform":"minimax","version":42}`)
	hub.Broadcast(broadcastMsg)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, recvMsg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, string(broadcastMsg), string(recvMsg))
}

func TestHandler_MultipleClients(t *testing.T) {
	hub := NewHub(func() {})

	handler := NewHandler(hub, Upgrader)

	router := gin.New()
	router.GET("/ws", handler.WSHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	go hub.Run()
	defer hub.Stop()

	u := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn1, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	assert.Equal(t, 2, clientCount)

	broadcastMsg := []byte("hello everyone")
	hub.Broadcast(broadcastMsg)

	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg1, err := conn1.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, string(broadcastMsg), string(msg1))

	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg2, err := conn2.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, string(broadcastMsg), string(msg2))
}

func TestHandler_InvalidUpgrade(t *testing.T) {
	hub := NewHub(func() {})

	handler := NewHandler(hub, Upgrader)

	router := gin.New()
	router.GET("/ws", handler.WSHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ws")
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}