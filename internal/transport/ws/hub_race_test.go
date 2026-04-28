package ws

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestHubRaceBroadcastRegisterUnregister tests concurrent broadcast while clients register/unregister.
func TestHubRaceBroadcastRegisterUnregister(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()
	defer hub.Stop()

	duration := 1 * time.Second
	stopCh := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]

	var wg sync.WaitGroup

	// Clients connecting and disconnecting
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					conn, _, err := websocket.DefaultDialer.Dial(u, nil)
					if err == nil {
						conn.Close()
					}
				}
			}
		}()
	}

	// Broadcasts
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					hub.Broadcast([]byte("broadcast message"))
				}
			}
		}()
	}

	// Active clients receiving broadcasts
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _, err := websocket.DefaultDialer.Dial(u, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.After(duration)
			for {
				select {
				case <-stopCh:
					return
				case <-timeout:
					return
				case <-ticker.C:
					conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
					conn.ReadMessage()
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()
}

// TestHubRaceStopDuringBroadcast tests stopping hub while broadcasts are in flight.
func TestHubRaceStopDuringBroadcast(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]

	// Connect a client
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		defer conn.Close()
	}

	var wg sync.WaitGroup

	// Broadcasters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				hub.Broadcast([]byte("message"))
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	hub.Stop()

	wg.Wait()
}

// TestHubRaceMultipleBroadcasts tests high concurrency broadcast scenarios.
func TestHubRaceMultipleBroadcasts(t *testing.T) {
	hub := NewHub(func() {})
	go hub.Run()
	defer hub.Stop()

	duration := 500 * time.Millisecond
	stopCh := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]

	var wg sync.WaitGroup
	var mu sync.Mutex
	receivedCount := 0

	// Clients receiving messages
	clients := make([]*websocket.Conn, 10)
	for i := 0; i < 10; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		if err == nil {
			clients[i] = conn
		}
	}

	// Broadcasters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					hub.Broadcast([]byte("broadcast"))
				}
			}
		}()
	}

	// Receivers reading
	for _, conn := range clients {
		if conn == nil {
			continue
		}
		conn := conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
					_, _, err := conn.ReadMessage()
					if err == nil {
						mu.Lock()
						receivedCount++
						mu.Unlock()
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	// Cleanup clients
	for _, conn := range clients {
		if conn != nil {
			conn.Close()
		}
	}

	t.Logf("Total messages received: %d", receivedCount)
}

// TestHubLeakGoroutines verifies no goroutine leaks after hub shutdown.
func TestHubLeakGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}

	before := runtime.NumGoroutine()

	hub := NewHub(func() {})
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(hub, conn)
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	defer server.Close()

	u := "ws" + server.URL[4:]

	// Connect clients
	clients := make([]*websocket.Conn, 5)
	for i := 0; i < 5; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		if err == nil {
			clients[i] = conn
		}
	}

	// Do some broadcasts
	for i := 0; i < 50; i++ {
		hub.Broadcast([]byte("test"))
	}

	// Cleanup
	for _, conn := range clients {
		if conn != nil {
			conn.Close()
		}
	}

	time.Sleep(50 * time.Millisecond)
	hub.Stop()

	// Allow cleanup time
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before

	if leaked > 5 {
		t.Errorf("goroutine leak detected: before=%d, after=%d, leaked=%d", before, after, leaked)
	}
}
