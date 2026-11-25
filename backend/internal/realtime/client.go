package realtime

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second    // max time to write one message
	pongWait       = 60 * time.Second    // client must answer pings within this
	pingPeriod     = (pongWait * 9) / 10 // how often we ping (must be < pongWait)
	maxMessageSize = 1024                // clients only send small heartbeats
	sendBuffer     = 64                  // queued messages per client before it counts as slow
)

// Client is one WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	remote string
}

// Handler upgrades GET /ws/alerts to a WebSocket and registers the client.
// allowOrigin decides which browser origins may connect.
func Handler(hub *Hub, allowOrigin func(origin string) bool, log *slog.Logger) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Non-browser clients (wscat, Postman, Go tests) send no Origin.
			return origin == "" || allowOrigin(origin)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade has already written an HTTP error response.
			log.Warn("websocket upgrade failed", "error", err, "origin", r.Header.Get("Origin"))
			return
		}
		c := &Client{hub: hub, conn: conn, send: make(chan []byte, sendBuffer), remote: r.RemoteAddr}

		select {
		case hub.register <- c:
		case <-hub.done: // server is shutting down
			_ = conn.Close()
			return
		}

		go c.writePump()
		go c.readPump()
	}
}

// readPump reads (and ignores) client messages so we notice pongs and
// disconnects. When it returns, the client is unregistered.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregisterClient(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
		// Any client message (e.g. an app-level heartbeat) also proves it's alive.
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// writePump is the only goroutine that writes to the connection. It sends
// queued alerts and periodic pings, and closes the connection when the hub
// closes the send channel.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// unregisterClient asks the hub to forget c. It returns immediately if the
// hub has already stopped (during shutdown).
func (h *Hub) unregisterClient(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}
