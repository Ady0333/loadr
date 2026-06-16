package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Hub maintains the set of connected clients and broadcasts messages to them.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
}

// client wraps a single WebSocket connection with a buffered send queue so a
// slow reader can't block the broadcaster.
type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewHub creates a Hub. The upgrader permits cross-origin connections so the
// Vite dev server (localhost:5173) can connect during development.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// Broadcast sends message to every connected client. Clients whose send queue
// is full are dropped so one stuck connection can't stall the others.
func (h *Hub) Broadcast(message []byte) {
	h.mu.RLock()
	var slow []*client
	for c := range h.clients {
		select {
		case c.send <- message:
		default:
			// Queue full: client is too slow to keep up.
			slow = append(slow, c)
		}
	}
	h.mu.RUnlock()

	// Drop slow clients outside the read lock; unregister closes their send
	// channel, which makes the write pump tear the connection down.
	for _, c := range slow {
		h.unregister(c)
	}
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS is the HTTP handler that upgrades a request to a WebSocket connection
// and registers the resulting client with the hub.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade failed: %v", err)
		return
	}

	c := &client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 64),
	}
	h.register(c)

	// Each connection runs an independent read and write pump.
	go c.writePump()
	go c.readPump()
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// readPump drains incoming messages (we don't expect any) and, more
// importantly, detects disconnects so the client can be cleaned up.
func (c *client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws: read error: %v", err)
			}
			return
		}
	}
}

// writePump pushes queued broadcasts to the connection and sends periodic pings
// to keep it alive and notice dead peers.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel: tell the peer we're done.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
