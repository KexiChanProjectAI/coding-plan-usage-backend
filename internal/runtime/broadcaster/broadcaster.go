package broadcaster

import (
	"context"
	"time"
)

// Broadcaster periodically invokes a callback until the context is cancelled.
type Broadcaster struct {
	interval  time.Duration
	broadcast func()
}

// New creates a periodic broadcaster.
func New(interval time.Duration, broadcast func()) *Broadcaster {
	return &Broadcaster{
		interval:  interval,
		broadcast: broadcast,
	}
}

// Start runs the broadcaster loop until ctx is done.
func (b *Broadcaster) Start(ctx context.Context) {
	if b == nil || b.broadcast == nil {
		return
	}

	interval := b.interval
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.broadcast()
		}
	}
}
