package routes

import (
	"database/sql"
	"net/http"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/xero7412/pick-me/backend/handlers"
	"github.com/xero7412/pick-me/backend/middleware"
	ws "github.com/xero7412/pick-me/backend/websocket"
)

func Register(r *gin.Engine, db *sql.DB, rdb *redis.Client, hub *ws.Hub, firebaseAuth *firebaseauth.Client) {
	authHandler := handlers.NewAuthHandler(db, firebaseAuth)
	friendsHandler := handlers.NewFriendsHandler(db)
	requestsHandler := handlers.NewRequestsHandler(db, rdb)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// WebSocket endpoint — auth via query param token during WS handshake
	r.GET("/ws", func(c *gin.Context) {
		uid := c.Query("uid")
		// TODO: verify token from query param before upgrading
		hub.ServeWS(uid, c.Writer, c.Request)
	})

	// Public routes — no auth middleware
	api := r.Group("/api")
	{
		api.POST("/auth/login", authHandler.Login)
	}

	// Protected routes — Firebase token required
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthRequired(firebaseAuth))
	{
		// Friends
		v1.POST("/friends/invite", friendsHandler.Invite)
		v1.POST("/friends/respond", friendsHandler.Respond)
		v1.GET("/friends", friendsHandler.List)
		v1.GET("/friends/pending", friendsHandler.Pending)

		// Pick-up requests
		v1.POST("/requests/send", requestsHandler.Send)
		v1.POST("/requests/respond", requestsHandler.Respond)
		v1.POST("/requests/complete", requestsHandler.Complete)
		v1.GET("/requests/active", requestsHandler.Active)
	}
}
