package models

import "time"

type User struct {
	ID          string    `json:"id" db:"id"`
	FirebaseUID string    `json:"firebase_uid" db:"firebase_uid"`
	DisplayName string    `json:"display_name" db:"display_name"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Friendship struct {
	ID         string    `json:"id" db:"id"`
	RequesterID string   `json:"requester_id" db:"requester_id"`
	AddresseeID string   `json:"addressee_id" db:"addressee_id"`
	Status     string    `json:"status" db:"status"` // pending, accepted, rejected
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type PickRequest struct {
	ID          string    `json:"id" db:"id"`
	RequesterID string    `json:"requester_id" db:"requester_id"`
	DriverID    string    `json:"driver_id" db:"driver_id"`
	PickupLat   float64   `json:"pickup_lat" db:"pickup_lat"`
	PickupLng   float64   `json:"pickup_lng" db:"pickup_lng"`
	DropLat     float64   `json:"drop_lat" db:"drop_lat"`
	DropLng     float64   `json:"drop_lng" db:"drop_lng"`
	Status      string    `json:"status" db:"status"` // pending, accepted, completed, cancelled
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
