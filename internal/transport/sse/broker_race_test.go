package sse

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestBrokerRacePublishSubscribe tests concurrent publish/subscribe/unsubscribe operations.
func TestBrokerRacePublishSubscribe(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	duration := 1 * time.Second
	stopCh := make(chan struct{})

	var wg sync.WaitGroup

	// Subscribers constantly subscribing/unsubscribing
	for i := 0; i < 5; i++ {
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
					ch := broker.Subscribe()
					broker.Unsubscribe(ch)
				}
			}
		}()
	}

	// Publishers constantly publishing
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					broker.Publish([]byte("test message"))
				}
			}
		}()
	}

	// Subscribers receiving messages
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := broker.Subscribe()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.After(duration)
			for {
				select {
				case <-stopCh:
					broker.Unsubscribe(ch)
					return
				case <-timeout:
					broker.Unsubscribe(ch)
					return
				case <-ticker.C:
					select {
					case <-ch:
					default:
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()
}

// TestBrokerRaceStopDuringPublish tests stopping broker while publishes are in flight.
func TestBrokerRaceStopDuringPublish(t *testing.T) {
	broker := New()
	go broker.Run()

	var wg sync.WaitGroup

	// Publishers trying to publish while we stop
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				broker.Publish([]byte("message"))
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	broker.Stop()

	wg.Wait()
}

// TestBrokerRaceMultipleSubscribers tests high concurrency subscriber scenarios.
func TestBrokerRaceMultipleSubscribers(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	duration := 500 * time.Millisecond
	stopCh := make(chan struct{})

	var wg sync.WaitGroup
	var mu sync.Mutex
	receivedCount := 0

	// Create many subscribers
	channels := make([]chan []byte, 20)
	for i := 0; i < 20; i++ {
		channels[i] = broker.Subscribe()
	}

	// Publishers
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
					broker.Publish([]byte("broadcast"))
				}
			}
		}()
	}

	// Receivers
	for _, ch := range channels {
		ch := ch
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
					select {
					case msg := <-ch:
						if msg != nil {
							mu.Lock()
							receivedCount++
							mu.Unlock()
						}
					default:
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	t.Logf("Total messages received by subscribers: %d", receivedCount)
}

// TestBrokerLeakGoroutines verifies no goroutine leaks after broker shutdown.
func TestBrokerLeakGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}

	before := runtime.NumGoroutine()

	broker := New()
	go broker.Run()

	// Create subscribers
	channels := make([]chan []byte, 10)
	for i := 0; i < 10; i++ {
		channels[i] = broker.Subscribe()
	}

	// Do some publishes
	for i := 0; i < 100; i++ {
		broker.Publish([]byte("test"))
	}

	// Cleanup subscribers
	for _, ch := range channels {
		broker.Unsubscribe(ch)
	}

	broker.Stop()

	// Allow cleanup time
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before

	if leaked > 5 {
		t.Errorf("goroutine leak detected: before=%d, after=%d, leaked=%d", before, after, leaked)
	}
}
