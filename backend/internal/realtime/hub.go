// Package realtime pushes alerts to browsers over WebSocket.
//
// One Hub goroutine owns the set of connected clients. Everything else talks
// to it through channels, so the client map needs no locks. Each client has
// its own buffered send queue and a writer goroutine, so one slow browser
// can never hold up the others or the HTTP request that produced the alert.
package realtime

import (
	"context"
	"log/slog"
	"sync/atomic"
)

const broadcastBuffer = 256

// Hub fans out messages to every connected client.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	clients    map[*Client]struct{}
	done       chan struct{} // closed when Run returns
	count      atomic.Int64
	log        *slog.Logger
}

// NewHub creates a hub. Call Run in its own goroutine.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, broadcastBuffer),
		clients:    make(map[*Client]struct{}),
		done:       make(chan struct{}),
		log:        log,
	}
}

// Run processes registrations and broadcasts until ctx is cancelled, then
// disconnects every client.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				h.remove(c)
			}
			return

		case c := <-h.register:
			h.clients[c] = struct{}{}
			h.count.Store(int64(len(h.clients)))
			h.log.Info("websocket client connected", "clients", len(h.clients), "remote", c.remote)

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				h.remove(c)
				h.log.Info("websocket client disconnected", "clients", len(h.clients), "remote", c.remote)
			}

		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// The client's queue is full: it can't keep up. Drop it so
					// it reconnects and re-syncs, instead of blocking everyone.
					h.remove(c)
					h.log.Warn("dropped slow websocket client", "remote", c.remote)
				}
			}
		}
	}
}

// Publish queues a message for every client. It never blocks: if the hub
// is overloaded the message is dropped (it is already stored in the
// database, so nothing is lost permanently).
func (h *Hub) Publish(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		h.log.Warn("alert broadcast queue full, dropping message")
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	return int(h.count.Load())
}

func (h *Hub) remove(c *Client) {
	delete(h.clients, c)
	close(c.send) // tells the client's writer goroutine to close the connection
	h.count.Store(int64(len(h.clients)))
}
