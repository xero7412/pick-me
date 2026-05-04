package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	UID  string
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client // keyed by uid
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]*Client)}
}

func (h *Hub) Register(uid string, conn *websocket.Conn) *Client {
	client := &Client{UID: uid, conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[uid] = client
	h.mu.Unlock()
	return client
}

func (h *Hub) Unregister(uid string) {
	h.mu.Lock()
	delete(h.clients, uid)
	h.mu.Unlock()
}

// Send pushes a message to a specific user if connected.
func (h *Hub) Send(uid string, msg []byte) {
	h.mu.RLock()
	client, ok := h.clients[uid]
	h.mu.RUnlock()
	if ok {
		client.send <- msg
	}
}

// ServeWS upgrades the HTTP connection and starts read/write pumps.
func (h *Hub) ServeWS(uid string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error for uid %s: %v", uid, err)
		return
	}

	client := h.Register(uid, conn)
	defer func() {
		h.Unregister(uid)
		conn.Close()
	}()

	go client.writePump()
	client.readPump()
}

func (c *Client) readPump() {
	defer close(c.send)
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// TODO: handle incoming client messages (e.g. location updates)
	}
}

func (c *Client) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("websocket write error for uid %s: %v", c.UID, err)
			return
		}
	}
}
