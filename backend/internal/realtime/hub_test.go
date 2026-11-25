package realtime

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startHub(t *testing.T) (*Hub, string) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(log)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	allow := func(origin string) bool { return origin == "https://allowed.example" }
	srv := httptest.NewServer(Handler(hub, allow, log))
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})
	return hub, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func waitForClients(t *testing.T, hub *Hub, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != n {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d clients, have %d", n, hub.ClientCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBroadcastReachesEveryClient(t *testing.T) {
	hub, url := startHub(t)

	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer c.Close()
		conns = append(conns, c)
	}
	waitForClients(t, hub, 3)

	hub.Publish([]byte(`{"event_id":"evt_1"}`))

	for i, c := range conns {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client %d: read: %v", i, err)
		}
		if string(msg) != `{"event_id":"evt_1"}` {
			t.Errorf("client %d got %s", i, msg)
		}
	}
}

func TestDisconnectedClientIsRemoved(t *testing.T) {
	hub, url := startHub(t)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitForClients(t, hub, 1)
	_ = c.Close()
	waitForClients(t, hub, 0)
}

func TestOriginCheck(t *testing.T) {
	_, url := startHub(t)

	header := map[string][]string{"Origin": {"https://evil.example"}}
	if _, _, err := websocket.DefaultDialer.Dial(url, header); err == nil {
		t.Error("expected a disallowed origin to be rejected")
	}

	header = map[string][]string{"Origin": {"https://allowed.example"}}
	c, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("allowed origin rejected: %v", err)
	}
	_ = c.Close()
}

func TestPublishNeverBlocks(t *testing.T) {
	// A hub that isn't running can't drain its queue; Publish must still return.
	hub := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() {
		for i := 0; i < broadcastBuffer*2; i++ {
			hub.Publish([]byte("x"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked")
	}
}
