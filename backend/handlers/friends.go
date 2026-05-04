package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FriendsHandler struct {
	DB *sql.DB
}

func NewFriendsHandler(db *sql.DB) *FriendsHandler {
	return &FriendsHandler{DB: db}
}

// SendRequest sends a friend request to another user.
func (h *FriendsHandler) SendRequest(c *gin.Context) {
	// TODO: create friendship row with status=pending
	c.JSON(http.StatusOK, gin.H{"message": "friend request sent"})
}

// RespondRequest accepts or rejects a pending friend request.
func (h *FriendsHandler) RespondRequest(c *gin.Context) {
	// TODO: update friendship status to accepted or rejected
	c.JSON(http.StatusOK, gin.H{"message": "friend request updated"})
}

// List returns the authenticated user's accepted friends.
func (h *FriendsHandler) List(c *gin.Context) {
	// TODO: query accepted friendships for current user
	c.JSON(http.StatusOK, gin.H{"friends": []any{}})
}
