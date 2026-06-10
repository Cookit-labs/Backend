package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// leaderboardKey is the special channel key used for global leaderboard subscribers.
const leaderboardKey = "__leaderboard__"

// Hub manages all active WebSocket connections and broadcasts messages to subscribers.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{} // channel key → set of clients
}

type Client struct {
	conn     *websocket.Conn
	intentID string
	send     chan []byte
}

type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(intentID string, conn *websocket.Conn) *Client {
	c := &Client{conn: conn, intentID: intentID, send: make(chan []byte, 64)}
	h.mu.Lock()
	if h.clients[intentID] == nil {
		h.clients[intentID] = make(map[*Client]struct{})
	}
	h.clients[intentID][c] = struct{}{}
	h.mu.Unlock()
	go c.writePump()
	return c
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients[c.intentID], c)
	if len(h.clients[c.intentID]) == 0 {
		delete(h.clients, c.intentID)
	}
	h.mu.Unlock()
	close(c.send)
}

// Broadcast sends a message to all clients subscribed to intentID.
func (h *Hub) Broadcast(intentID string, msgType string, payload any) {
	data, err := json.Marshal(Message{Type: msgType, Payload: payload})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[intentID] {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// RegisterLeaderboard subscribes a WebSocket connection to global leaderboard updates.
func (h *Hub) RegisterLeaderboard(conn *websocket.Conn) *Client {
	return h.Register(leaderboardKey, conn)
}

// BroadcastLeaderboard sends a message to all leaderboard subscribers.
func (h *Hub) BroadcastLeaderboard(msgType string, payload any) {
	h.Broadcast(leaderboardKey, msgType, payload)
}

// ReadPump blocks, reading from the connection until it closes (ping/close frames).
// Call this from the HTTP handler goroutine after registering the client.
func (h *Hub) ReadPump(c *Client) {
	defer h.Unregister(c)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}
