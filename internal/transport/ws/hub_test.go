package ws

import (
	"net/http"
	"net/http/httptest"
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

func TestHub_ClientConnectAndReceiveBroadcast(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	testMsg := []byte("hello world")
	hub.Broadcast(testMsg)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, recvMsg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(recvMsg))
}

func TestHub_RefreshMessageTriggersCallback(t *testing.T) {
	var mu sync.Mutex
	refreshCallCount := 0

	hub := NewHub(func() {
		mu.Lock()
		refreshCallCount++
		mu.Unlock()
	})

	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]
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

func TestHub_DisconnectRemovesClient(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	assert.Equal(t, 1, clientCount)

	conn.Close()

	time.Sleep(200 * time.Millisecond)

	hub.mu.RLock()
	clientCount = len(hub.clients)
	hub.mu.RUnlock()
	assert.Equal(t, 0, clientCount)
}

func TestHub_StopCleansUpClients(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]
	conn1, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	conn2, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	hub.Stop()
	conn1.Close()
	conn2.Close()

	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	assert.Equal(t, 0, clientCount)
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]
	conn1, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	conn2, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	testMsg := []byte("broadcast test")
	hub.Broadcast(testMsg)

	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg1, err := conn1.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "broadcast test", string(msg1))

	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg2, err := conn2.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "broadcast test", string(msg2))

	conn1.Close()
	conn2.Close()
}