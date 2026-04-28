package sse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockBroker struct {
	mu       sync.RWMutex
	messages [][]byte
	subs     []chan []byte
}

func (m *mockBroker) Subscribe() chan []byte {
	ch := make(chan []byte, 256)
	m.mu.Lock()
	m.subs = append(m.subs, ch)
	m.mu.Unlock()
	return ch
}

func (m *mockBroker) Unsubscribe(ch chan []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.subs {
		if s == ch {
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			close(ch)
			break
		}
	}
}

func (m *mockBroker) Publish(msg []byte) {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()
	for _, ch := range m.subs {
		ch <- msg
	}
}

func TestStreamHandler_SetsHeaders(t *testing.T) {
	broker := &mockBroker{}
	handler := NewHandler(broker)

	router := gin.New()
	router.GET("/stream", handler.StreamHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/stream", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
}

func TestStreamHandler_ReceivesEvent(t *testing.T) {
	broker := &mockBroker{}
	handler := NewHandler(broker)

	router := gin.New()
	router.GET("/stream", handler.StreamHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/stream", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	broker.Publish([]byte(`{"platform":"test","version":1}`))

	reader := io.LimitReader(resp.Body, 1024)
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	require.NoError(t, err)

	msg := string(buf[:n])
	assert.Contains(t, msg, "data: ")
	assert.Contains(t, msg, `{"platform":"test","version":1}`)
}

func TestStreamHandler_ClientDisconnect(t *testing.T) {
	broker := &mockBroker{}
	handler := NewHandler(broker)

	router := gin.New()
	router.GET("/stream", handler.StreamHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/stream", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	broker.mu.RLock()
	subCount := len(broker.subs)
	broker.mu.RUnlock()
	assert.Equal(t, 1, subCount)

	cancel()

	time.Sleep(100 * time.Millisecond)

	broker.mu.RLock()
	subCount = len(broker.subs)
	broker.mu.RUnlock()
	assert.Equal(t, 0, subCount)
}

func TestStreamHandler_MultipleClients(t *testing.T) {
	broker := &mockBroker{}
	handler := NewHandler(broker)

	router := gin.New()
	router.GET("/stream", handler.StreamHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	req1, _ := http.NewRequest("GET", server.URL+"/stream", nil)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()

	req2, _ := http.NewRequest("GET", server.URL+"/stream", nil)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	time.Sleep(50 * time.Millisecond)

	broker.mu.RLock()
	subCount := len(broker.subs)
	broker.mu.RUnlock()
	assert.Equal(t, 2, subCount)

	broker.Publish([]byte(`test`))

	resp1.Body.Close()
	resp2.Body.Close()
}

func TestStreamHandler_FormattedData(t *testing.T) {
	broker := &mockBroker{}
	handler := NewHandler(broker)

	router := gin.New()
	router.GET("/stream", handler.StreamHandler())

	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	broker.Publish([]byte(`{"platform":"minimax","version":42}`))

	reader := io.LimitReader(resp.Body, 1024)
	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)

	msg := string(buf[:n])
	assert.True(t, strings.HasPrefix(msg, "data: "))
	assert.Contains(t, msg, "\n\n")
	assert.Contains(t, msg, `{"platform":"minimax","version":42}`)
}