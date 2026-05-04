package routes

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	firebaseauth "firebase.google.com/go/v4/auth"

	"github.com/xero7412/pick-me/backend/handlers"
	"github.com/xero7412/pick-me/backend/middleware"
	ws "github.com/xero7412/pick-me/backend/websocket"
)

func Register(r *gin.Engine, db *sql.DB, hub *ws.Hub, firebaseAuth *firebaseauth.Client) {
	authHandler := handlers.NewAuthHandler(db)
	friendsHandler := handlers.NewFriendsHandler(db)
	requestsHandler := handlers.NewRequestsHandler(db)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// WebSocket endpoint (auth via query param token for WS handshake)
	r.GET("/ws", func(c *gin.Context) {
		uid := c.Query("uid")
		// TODO: verify token from query param before upgrading
		hub.ServeWS(uid, c.Writer, c.Request)
	})

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(firebaseAuth))
	{
		// Auth
		api.POST("/auth/register", authHandler.Register)
		api.GET("/auth/me", authHandler.Me)

		// Friends
		api.POST("/friends/request", friendsHandler.SendRequest)
		api.PUT("/friends/request/:id", friendsHandler.RespondRequest)
		api.GET("/friends", friendsHandler.List)

		// Pick requests
		api.POST("/requests", requestsHandler.Create)
		api.PUT("/requests/:id/accept", requestsHandler.Accept)
		api.PUT("/requests/:id/complete", requestsHandler.Complete)
		api.PUT("/requests/:id/cancel", requestsHandler.Cancel)
		api.GET("/requests", requestsHandler.List)
	}
}
