package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/xero7412/pick-me/backend/config"
	"github.com/xero7412/pick-me/backend/db"
	"github.com/xero7412/pick-me/backend/firebase"
	"github.com/xero7412/pick-me/backend/routes"
	redisclient "github.com/xero7412/pick-me/backend/redis"
	ws "github.com/xero7412/pick-me/backend/websocket"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg.DatabaseURL)
	defer database.Close()

	rdb := redisclient.Connect(cfg.RedisURL)

	fbClient := firebase.Init(cfg.FirebaseCredentialsPath)

	hub := ws.NewHub()
	go hub.Run()

	r := gin.Default()
	routes.Register(r, database, rdb, hub, fbClient.Auth)

	log.Printf("starting Pick Me server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
