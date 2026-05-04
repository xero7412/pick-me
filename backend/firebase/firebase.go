package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Client struct {
	Auth *auth.Client
}

func Init(credentialsPath string) *Client {
	opts := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(context.Background(), nil, opts)
	if err != nil {
		log.Fatalf("failed to initialise Firebase app: %v", err)
	}

	authClient, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("failed to initialise Firebase Auth client: %v", err)
	}

	log.Println("Firebase initialised")
	return &Client{Auth: authClient}
}
