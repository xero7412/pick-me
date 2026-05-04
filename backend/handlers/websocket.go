package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	ws "github.com/xero7412/pick-me/backend/websocket"
)

type WebSocketHandler struct {
	DB  *sql.DB
	Hub *ws.Hub
}

func NewWebSocketHandler(db *sql.DB, hub *ws.Hub) *WebSocketHandler {
	return &WebSocketHandler{DB: db, Hub: hub}
}

// ConnectToRoom upgrades the connection and joins the user to the room for a given request.
// Query params: request_id (UUID of an accepted pickup request)
func (h *WebSocketHandler) ConnectToRoom(c *gin.Context) {
	requestID := c.Query("request_id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id query param required"})
		return
	}

	userID := c.GetString("uid")

	var requesterID, receiverID, status string
	err := h.DB.QueryRowContext(c.Request.Context(),
		`SELECT requester_id, receiver_id, status FROM pickup_requests WHERE id = $1`,
		requestID,
	).Scan(&requesterID, &receiverID, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if err != nil {
		log.Printf("ws connect: fetch request %s: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if userID != requesterID && userID != receiverID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorised"})
		return
	}
	if status != "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request is not active"})
		return
	}

	ws.ServeWS(h.Hub, userID, requestID, c.Writer, c.Request)
}
