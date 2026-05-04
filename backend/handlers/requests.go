package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/xero7412/pick-me/backend/models"
)

type RequestsHandler struct {
	DB    *sql.DB
	Redis *redis.Client
}

func NewRequestsHandler(db *sql.DB, rdb *redis.Client) *RequestsHandler {
	return &RequestsHandler{DB: db, Redis: rdb}
}

// requestCols is the fixed column order used in every SELECT / RETURNING clause.
const requestCols = `id, requester_id, receiver_id, status, requester_lat, requester_lng, created_at, responded_at, completed_at`

func scanPickupRequest(row *sql.Row) (models.PickupRequest, error) {
	var r models.PickupRequest
	err := row.Scan(
		&r.ID, &r.RequesterID, &r.ReceiverID, &r.Status,
		&r.RequesterLat, &r.RequesterLng, &r.CreatedAt,
		&r.RespondedAt, &r.CompletedAt,
	)
	return r, err
}

func activeKey(userID string) string {
	return fmt.Sprintf("active_request:%s", userID)
}

func insertNotif(ctx context.Context, db *sql.DB, userID, notifType string, payload any) {
	b, _ := json.Marshal(payload)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, $2, $3)`,
		userID, notifType, b,
	); err != nil {
		log.Printf("insertNotif(%s, %s): %v", userID, notifType, err)
	}
}

func hasActiveRequest(ctx context.Context, db *sql.DB, userID string) (bool, error) {
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM pickup_requests
		 WHERE (requester_id = $1 OR receiver_id = $1)
		   AND status IN ('pending', 'accepted')
		 LIMIT 1`,
		userID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- Send ---

type sendRequest struct {
	ReceiverID string  `json:"receiver_id" binding:"required"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
}

func (h *RequestsHandler) Send(c *gin.Context) {
	requesterID := c.GetString("uid")

	var req sendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "receiver_id, lat and lng are required"})
		return
	}

	// Verify accepted friendship exists in either direction
	var friendshipID string
	err := h.DB.QueryRowContext(c.Request.Context(),
		`SELECT id FROM friendships
		 WHERE ((requester_id = $1 AND receiver_id = $2) OR (requester_id = $2 AND receiver_id = $1))
		   AND status = 'accepted'`,
		requesterID, req.ReceiverID,
	).Scan(&friendshipID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only request friends"})
		return
	}
	if err != nil {
		log.Printf("send: check friendship: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Check requester has no active request
	busy, err := hasActiveRequest(c.Request.Context(), h.DB, requesterID)
	if err != nil {
		log.Printf("send: check requester active: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if busy {
		c.JSON(http.StatusConflict, gin.H{"error": "you already have an active request"})
		return
	}

	// Check receiver has no active request
	busy, err = hasActiveRequest(c.Request.Context(), h.DB, req.ReceiverID)
	if err != nil {
		log.Printf("send: check receiver active: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if busy {
		c.JSON(http.StatusConflict, gin.H{"error": "your friend is busy, try again later"})
		return
	}

	// Create pickup request
	pr, err := scanPickupRequest(h.DB.QueryRowContext(c.Request.Context(),
		`INSERT INTO pickup_requests (requester_id, receiver_id, status, requester_lat, requester_lng)
		 VALUES ($1, $2, 'pending', $3, $4)
		 RETURNING `+requestCols,
		requesterID, req.ReceiverID, req.Lat, req.Lng,
	))
	if err != nil {
		log.Printf("send: insert pickup_request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Cache request id in Redis with 5-minute TTL (request expires if not responded to)
	ctx := c.Request.Context()
	ttl := 5 * time.Minute
	h.Redis.Set(ctx, activeKey(requesterID), pr.ID, ttl)
	h.Redis.Set(ctx, activeKey(req.ReceiverID), pr.ID, ttl)

	// Fetch requester info for notification payload
	var name string
	var avatar sql.NullString
	_ = h.DB.QueryRowContext(ctx,
		`SELECT name, avatar_url FROM users WHERE id = $1`, requesterID,
	).Scan(&name, &avatar)

	insertNotif(ctx, h.DB, req.ReceiverID, "pickup_request", map[string]any{
		"request_id":       pr.ID,
		"requester_name":   name,
		"requester_avatar": avatar.String,
		"lat":              req.Lat,
		"lng":              req.Lng,
	})

	c.JSON(http.StatusCreated, pr)
}

// --- Respond ---

type respondPickupRequest struct {
	RequestID string `json:"request_id" binding:"required"`
	Action    string `json:"action" binding:"required"`
}

func (h *RequestsHandler) Respond(c *gin.Context) {
	receiverID := c.GetString("uid")

	var req respondPickupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id and action are required"})
		return
	}
	if req.Action != "accepted" && req.Action != "declined" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'accepted' or 'declined'"})
		return
	}

	pr, err := scanPickupRequest(h.DB.QueryRowContext(c.Request.Context(),
		`SELECT `+requestCols+` FROM pickup_requests WHERE id = $1`, req.RequestID,
	))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if err != nil {
		log.Printf("respond: fetch request %s: %v", req.RequestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if pr.ReceiverID != receiverID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorised"})
		return
	}
	if pr.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request already responded to"})
		return
	}

	pr, err = scanPickupRequest(h.DB.QueryRowContext(c.Request.Context(),
		`UPDATE pickup_requests SET status = $1, responded_at = now()
		 WHERE id = $2 RETURNING `+requestCols,
		req.Action, req.RequestID,
	))
	if err != nil {
		log.Printf("respond: update request %s: %v", req.RequestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Fetch receiver info for notification payload
	var name string
	var avatar sql.NullString
	_ = h.DB.QueryRowContext(c.Request.Context(),
		`SELECT name, avatar_url FROM users WHERE id = $1`, receiverID,
	).Scan(&name, &avatar)

	ctx := c.Request.Context()
	if req.Action == "declined" {
		h.Redis.Del(ctx, activeKey(pr.RequesterID), activeKey(pr.ReceiverID))
		insertNotif(ctx, h.DB, pr.RequesterID, "request_declined", map[string]any{
			"request_id":    pr.ID,
			"receiver_name": name,
		})
	} else {
		// Extend TTL to 24 hours — request is now active until completed or cancelled
		h.Redis.Expire(ctx, activeKey(pr.RequesterID), 24*time.Hour)
		h.Redis.Expire(ctx, activeKey(pr.ReceiverID), 24*time.Hour)
		insertNotif(ctx, h.DB, pr.RequesterID, "request_accepted", map[string]any{
			"request_id":      pr.ID,
			"receiver_name":   name,
			"receiver_avatar": avatar.String,
		})
	}

	c.JSON(http.StatusOK, pr)
}

// --- Complete ---

type completeRequest struct {
	RequestID string `json:"request_id" binding:"required"`
}

func (h *RequestsHandler) Complete(c *gin.Context) {
	receiverID := c.GetString("uid")

	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id is required"})
		return
	}

	pr, err := scanPickupRequest(h.DB.QueryRowContext(c.Request.Context(),
		`SELECT `+requestCols+` FROM pickup_requests WHERE id = $1`, req.RequestID,
	))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if err != nil {
		log.Printf("complete: fetch request %s: %v", req.RequestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if pr.ReceiverID != receiverID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorised"})
		return
	}
	if pr.Status != "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request is not active"})
		return
	}

	pr, err = scanPickupRequest(h.DB.QueryRowContext(c.Request.Context(),
		`UPDATE pickup_requests SET status = 'completed', completed_at = now()
		 WHERE id = $1 RETURNING `+requestCols,
		req.RequestID,
	))
	if err != nil {
		log.Printf("complete: update request %s: %v", req.RequestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	ctx := c.Request.Context()
	h.Redis.Del(ctx, activeKey(pr.RequesterID), activeKey(pr.ReceiverID))

	var name string
	_ = h.DB.QueryRowContext(ctx, `SELECT name FROM users WHERE id = $1`, receiverID).Scan(&name)
	insertNotif(ctx, h.DB, pr.RequesterID, "arrived", map[string]any{
		"request_id":    pr.ID,
		"receiver_name": name,
	})

	c.JSON(http.StatusOK, pr)
}

// --- Active ---

func (h *RequestsHandler) Active(c *gin.Context) {
	userID := c.GetString("uid")
	ctx := c.Request.Context()

	requestID, err := h.Redis.Get(ctx, activeKey(userID)).Result()
	if err == redis.Nil {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}
	if err != nil {
		log.Printf("active: redis get for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	pr, err := scanPickupRequest(h.DB.QueryRowContext(ctx,
		`SELECT `+requestCols+` FROM pickup_requests WHERE id = $1`, requestID,
	))
	if err != nil {
		log.Printf("active: fetch request %s: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"active": true, "request": pr})
}
