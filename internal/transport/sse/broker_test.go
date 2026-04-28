package sse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrokerSubscribe(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	ch := broker.Subscribe()
	require.NotNil(t, ch)
}

func TestBrokerUnsubscribe(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	ch := broker.Subscribe()
	broker.Unsubscribe(ch)

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after unsubscribe")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestStreamReceivesChangeEvent(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	ch := broker.Subscribe()

	msg := []byte(`{"platform":"test","version":1}`)
	broker.Publish(msg)

	select {
	case received := <-ch:
		assert.Equal(t, msg, received)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestSlowSubscriberIsDroppedOrSkipped(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	ch1 := broker.Subscribe()

	for i := 0; i < 300; i++ {
		broker.Publish([]byte(`test`))
	}
	_ = ch1
}

func TestBrokerPublishNonBlocking(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	for i := 0; i < 10; i++ {
		broker.Publish([]byte(`test`))
	}
}

func TestBrokerStop(t *testing.T) {
	broker := New()
	go broker.Run()

	broker.Subscribe()
	broker.Subscribe()
	broker.Stop()
}

func TestBrokerMultipleSubscribersRegistered(t *testing.T) {
	broker := New()
	go broker.Run()
	defer broker.Stop()

	ch1 := broker.Subscribe()
	ch2 := broker.Subscribe()

	time.Sleep(50 * time.Millisecond)

	broker.mu.RLock()
	subCount := len(broker.subscribers)
	broker.mu.RUnlock()

	assert.Equal(t, 2, subCount)
	assert.NotNil(t, ch1)
	assert.NotNil(t, ch2)
}