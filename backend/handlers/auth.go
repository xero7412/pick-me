package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	DB *sql.DB
}

func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

// Register creates or fetches a user record after Firebase auth.
func (h *AuthHandler) Register(c *gin.Context) {
	// TODO: extract uid from context, upsert user in DB
	c.JSON(http.StatusOK, gin.H{"message": "register"})
}

// Me returns the authenticated user's profile.
func (h *AuthHandler) Me(c *gin.Context) {
	// TODO: fetch user by uid from DB and return profile
	c.JSON(http.StatusOK, gin.H{"message": "me"})
}
