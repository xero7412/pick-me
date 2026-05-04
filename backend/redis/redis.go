package redis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func Connect(redisURL string) *redis.Client {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("failed to parse Redis URL: %v", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to ping Redis: %v", err)
	}

	log.Println("connected to Redis")
	return client
}
