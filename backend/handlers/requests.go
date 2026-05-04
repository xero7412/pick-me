package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RequestsHandler struct {
	DB *sql.DB
}

func NewRequestsHandler(db *sql.DB) *RequestsHandler {
	return &RequestsHandler{DB: db}
}

// Create creates a new pick-up request.
func (h *RequestsHandler) Create(c *gin.Context) {
	// TODO: validate body, insert PickRequest row, notify driver via WebSocket
	c.JSON(http.StatusCreated, gin.H{"message": "pick request created"})
}

// Accept allows a driver to accept a pending request.
func (h *RequestsHandler) Accept(c *gin.Context) {
	// TODO: update status to accepted, notify requester via WebSocket
	c.JSON(http.StatusOK, gin.H{"message": "request accepted"})
}

// Complete marks a request as completed.
func (h *RequestsHandler) Complete(c *gin.Context) {
	// TODO: update status to completed
	c.JSON(http.StatusOK, gin.H{"message": "request completed"})
}

// Cancel cancels an active request.
func (h *RequestsHandler) Cancel(c *gin.Context) {
	// TODO: update status to cancelled, notify relevant party via WebSocket
	c.JSON(http.StatusOK, gin.H{"message": "request cancelled"})
}

// List returns pick requests for the authenticated user.
func (h *RequestsHandler) List(c *gin.Context) {
	// TODO: query requests where requester_id or driver_id = uid
	c.JSON(http.StatusOK, gin.H{"requests": []any{}})
}
