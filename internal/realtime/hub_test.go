package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHubRoutesByExamAndDropsSlowClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.Run(ctx)

	a := &client{examID: 1, send: make(chan []byte, 8)}
	b := &client{examID: 2, send: make(chan []byte, 8)}
	slow := &client{examID: 1, send: make(chan []byte)} // unbuffered: always "slow"
	h.register <- a
	h.register <- b
	h.register <- slow

	// Concurrent publishers must be safe (run with -race where cgo is available).
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); h.Publish(1, "violation", map[string]int{"n": 1}) }()
	}
	wg.Wait()

	for i := 0; i < 4; i++ {
		select {
		case msg := <-a.send:
			var ev Event
			require.NoError(t, json.Unmarshal(msg, &ev))
			assert.Equal(t, int64(1), ev.ExamID)
		case <-time.After(time.Second):
			t.Fatal("client in exam 1 did not receive event")
		}
	}
	select {
	case <-b.send:
		t.Fatal("exam 2 client received exam 1 event")
	default:
	}
	// The slow client was removed and its channel closed.
	select {
	case _, ok := <-slow.send:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("slow client was not dropped")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	_, ok := <-a.send
	assert.False(t, ok, "shutdown closes client channels")
}
