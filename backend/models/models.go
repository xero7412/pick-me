package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID        string         `json:"id"`
	GoogleID  string         `json:"google_id"`
	Name      string         `json:"name"`
	Email     string         `json:"email"`
	AvatarURL sql.NullString `json:"avatar_url"`
	CreatedAt time.Time      `json:"created_at"`
}

type Friendship struct {
	ID          string    `json:"id"`
	RequesterID string    `json:"requester_id"`
	ReceiverID  string    `json:"receiver_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PickupRequest struct {
	ID           string         `json:"id"`
	RequesterID  string         `json:"requester_id"`
	ReceiverID   string         `json:"receiver_id"`
	Status       string         `json:"status"`
	RequesterLat float64        `json:"requester_lat"`
	RequesterLng float64        `json:"requester_lng"`
	CreatedAt    time.Time      `json:"created_at"`
	RespondedAt  sql.NullTime   `json:"responded_at"`
	CompletedAt  sql.NullTime   `json:"completed_at"`
}

type Notification struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Type      string         `json:"type"`
	Payload   []byte         `json:"payload"`
	IsRead    bool           `json:"is_read"`
	CreatedAt time.Time      `json:"created_at"`
}
