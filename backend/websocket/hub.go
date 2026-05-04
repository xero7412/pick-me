package websocket

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // must be less than pongWait
	maxMessageSize = 1024
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client represents one connected user inside a room.
type Client struct {
	UserID    string
	RequestID string
	conn      *websocket.Conn
	send      chan []byte
}

// Room holds all clients connected to the same pick-up request.
type Room struct {
	RequestID string
	clients   map[string]*Client // keyed by UserID
}

type broadcastMsg struct {
	roomID   string
	senderID string
	data     []byte
}

// Hub manages all active rooms via a single Run() goroutine — no mutexes needed.
type Hub struct {
	rooms      map[string]*Room // keyed by requestID
	register   chan *Client
	unregister chan *Client
	broadcast  chan *broadcastMsg
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *broadcastMsg),
	}
}

// Run processes hub events sequentially — call as a goroutine in main.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			room, ok := h.rooms[client.RequestID]
			if !ok {
				room = &Room{
					RequestID: client.RequestID,
					clients:   make(map[string]*Client),
				}
				h.rooms[client.RequestID] = room
			}
			room.clients[client.UserID] = client
			log.Printf("ws: user %s joined room %s (%d clients)", client.UserID, client.RequestID, len(room.clients))

		case client := <-h.unregister:
			room, ok := h.rooms[client.RequestID]
			if !ok {
				continue
			}
			if _, exists := room.clients[client.UserID]; exists {
				delete(room.clients, client.UserID)
				close(client.send)
				log.Printf("ws: user %s left room %s (%d clients)", client.UserID, client.RequestID, len(room.clients))
			}
			if len(room.clients) == 0 {
				delete(h.rooms, client.RequestID)
				log.Printf("ws: room %s closed", client.RequestID)
			}

		case msg := <-h.broadcast:
			room, ok := h.rooms[msg.roomID]
			if !ok {
				continue
			}
			for uid, client := range room.clients {
				if uid == msg.senderID {
					continue // don't echo back to sender
				}
				select {
				case client.send <- msg.data:
				default:
					// Slow client — drop message to avoid blocking the hub
					log.Printf("ws: dropped message to slow client %s in room %s", uid, msg.roomID)
				}
			}
		}
	}
}

// ServeWS upgrades the connection and wires the client into the hub.
func ServeWS(hub *Hub, userID, requestID string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error user=%s room=%s: %v", userID, requestID, err)
		return
	}

	client := &Client{
		UserID:    userID,
		RequestID: requestID,
		conn:      conn,
		send:      make(chan []byte, 64),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump(hub)
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws: read error user=%s: %v", c.UserID, err)
			}
			break
		}
		hub.broadcast <- &broadcastMsg{
			roomID:   c.RequestID,
			senderID: c.UserID,
			data:     data,
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("ws: write error user=%s: %v", c.UserID, err)
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
