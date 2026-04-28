// Package sse provides Server-Sent Events transport for real-time quota updates.
package sse

import (
	"log"
	"sync"
)

// Broker maintains a set of subscribers and broadcasts messages to all of them.
// It uses non-blocking sends to avoid blocking pollers on slow clients.
type Broker struct {
	subscribers map[chan []byte]bool
	mu          sync.RWMutex
	register    chan chan []byte
	unregister  chan chan []byte
	broadcast   chan []byte
	done        chan struct{}
	stopOnce    sync.Once
}

// Event is the SSE payload wrapper used for versioned snapshot streams.
type Event struct {
	Version  int64       `json:"version"`
	Snapshot interface{} `json:"snapshot"`
}

// New creates a new Broker instance.
func New() *Broker {
	return &Broker{
		subscribers: make(map[chan []byte]bool),
		register:    make(chan chan []byte),
		unregister:  make(chan chan []byte),
		broadcast:   make(chan []byte),
		done:        make(chan struct{}),
	}
}

// Run starts the broker's main loop processing register/unregister/broadcast events.
func (b *Broker) Run() {
	for {
		select {
		case ch := <-b.register:
			b.mu.Lock()
			b.subscribers[ch] = true
			b.mu.Unlock()

		case ch := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.subscribers[ch]; ok {
				delete(b.subscribers, ch)
				close(ch)
			}
			b.mu.Unlock()

		case msg := <-b.broadcast:
			b.mu.Lock()
			for ch := range b.subscribers {
				select {
				case ch <- msg:
				default:
					// Subscriber buffer full, drop and close.
					delete(b.subscribers, ch)
					close(ch)
					log.Println("sse/broker: subscriber buffer full, dropping")
				}
			}
			b.mu.Unlock()

		case <-b.done:
			// Clean up all subscribers on shutdown.
			b.mu.Lock()
			for ch := range b.subscribers {
				delete(b.subscribers, ch)
				close(ch)
			}
			b.mu.Unlock()
			return
		}
	}
}

// Subscribe registers a new subscriber and returns the channel to receive messages.
func (b *Broker) Subscribe() chan []byte {
	ch := make(chan []byte, 256)
	select {
	case b.register <- ch:
	case <-b.done:
	}
	return ch
}

// Unsubscribe removes a subscriber from the broker.
func (b *Broker) Unsubscribe(ch chan []byte) {
	select {
	case b.unregister <- ch:
	case <-b.done:
	}
}

// Publish sends a message to all registered subscribers (non-blocking).
func (b *Broker) Publish(msg []byte) {
	select {
	case b.broadcast <- msg:
	default:
		// Broadcast channel full, log and continue.
		log.Println("sse/broker: broadcast channel full, dropping message")
	}
}

// Stop shuts down the broker gracefully.
func (b *Broker) Stop() {
	b.stopOnce.Do(func() {
		close(b.done)
	})
}
