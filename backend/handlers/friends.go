package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xero7412/pick-me/backend/models"
)

type FriendsHandler struct {
	DB *sql.DB
}

func NewFriendsHandler(db *sql.DB) *FriendsHandler {
	return &FriendsHandler{DB: db}
}

// currentUserID reads the uid set by auth middleware.
// When Firebase is configured, uid comes from the verified token.
// When Firebase is nil (dev bypass), uid comes from the X-User-ID header via middleware.
// TODO: once all handlers are wired to real Firebase auth, inline c.GetString("uid") directly.
func currentUserID(c *gin.Context) string {
	return c.GetString("uid")
}

// --- Invite ---

type inviteRequest struct {
	Email string `json:"email" binding:"required"`
}

func (h *FriendsHandler) Invite(c *gin.Context) {
	senderID := currentUserID(c)
	if senderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header required"})
		return
	}

	var req inviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	// Find receiver by email
	var receiverID string
	err := h.DB.QueryRowContext(c.Request.Context(),
		`SELECT id FROM users WHERE email = $1`, req.Email,
	).Scan(&receiverID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found — they must have the app installed"})
		return
	}
	if err != nil {
		log.Printf("invite: lookup receiver by email %s: %v", req.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Check for existing friendship in either direction
	var existingID string
	err = h.DB.QueryRowContext(c.Request.Context(),
		`SELECT id FROM friendships
		 WHERE (requester_id = $1 AND receiver_id = $2)
		    OR (requester_id = $2 AND receiver_id = $1)`,
		senderID, receiverID,
	).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("invite: check existing friendship: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if existingID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "friendship already exists"})
		return
	}

	// Create friendship
	var f models.Friendship
	err = h.DB.QueryRowContext(c.Request.Context(),
		`INSERT INTO friendships (requester_id, receiver_id, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id, requester_id, receiver_id, status, created_at, updated_at`,
		senderID, receiverID,
	).Scan(&f.ID, &f.RequesterID, &f.ReceiverID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		log.Printf("invite: create friendship: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Fetch sender info for notification payload
	var senderName, senderEmail string
	var senderAvatar sql.NullString
	_ = h.DB.QueryRowContext(c.Request.Context(),
		`SELECT name, email, avatar_url FROM users WHERE id = $1`, senderID,
	).Scan(&senderName, &senderEmail, &senderAvatar)

	payload, _ := json.Marshal(map[string]any{
		"friendship_id":  f.ID,
		"sender_name":    senderName,
		"sender_email":   senderEmail,
		"sender_avatar":  senderAvatar.String,
	})
	if _, err = h.DB.ExecContext(c.Request.Context(),
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, 'friend_invite', $2)`,
		receiverID, payload,
	); err != nil {
		// Friendship was created — don't fail the request over the notification
		log.Printf("invite: create notification for receiver %s: %v", receiverID, err)
	}

	c.JSON(http.StatusCreated, f)
}

// --- Respond ---

type respondRequest struct {
	FriendshipID string `json:"friendship_id" binding:"required"`
	Action       string `json:"action" binding:"required"`
}

func (h *FriendsHandler) Respond(c *gin.Context) {
	receiverID := currentUserID(c)
	if receiverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header required"})
		return
	}

	var req respondRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "friendship_id and action are required"})
		return
	}
	if req.Action != "accepted" && req.Action != "declined" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'accepted' or 'declined'"})
		return
	}

	// Fetch friendship
	var f models.Friendship
	err := h.DB.QueryRowContext(c.Request.Context(),
		`SELECT id, requester_id, receiver_id, status, created_at, updated_at
		 FROM friendships WHERE id = $1`,
		req.FriendshipID,
	).Scan(&f.ID, &f.RequesterID, &f.ReceiverID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "friendship not found"})
		return
	}
	if err != nil {
		log.Printf("respond: fetch friendship %s: %v", req.FriendshipID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if f.ReceiverID != receiverID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorised"})
		return
	}
	if f.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "friendship already responded to"})
		return
	}

	// Update status
	err = h.DB.QueryRowContext(c.Request.Context(),
		`UPDATE friendships SET status = $1, updated_at = now()
		 WHERE id = $2
		 RETURNING id, requester_id, receiver_id, status, created_at, updated_at`,
		req.Action, req.FriendshipID,
	).Scan(&f.ID, &f.RequesterID, &f.ReceiverID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		log.Printf("respond: update friendship %s: %v", req.FriendshipID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Notify requester if accepted
	if req.Action == "accepted" {
		var receiverName string
		var receiverAvatar sql.NullString
		_ = h.DB.QueryRowContext(c.Request.Context(),
			`SELECT name, avatar_url FROM users WHERE id = $1`, receiverID,
		).Scan(&receiverName, &receiverAvatar)

		payload, _ := json.Marshal(map[string]any{
			"friendship_id":   f.ID,
			"receiver_name":   receiverName,
			"receiver_avatar": receiverAvatar.String,
		})
		if _, err = h.DB.ExecContext(c.Request.Context(),
			`INSERT INTO notifications (user_id, type, payload) VALUES ($1, 'friend_invite_accepted', $2)`,
			f.RequesterID, payload,
		); err != nil {
			log.Printf("respond: create notification for requester %s: %v", f.RequesterID, err)
		}
	}

	c.JSON(http.StatusOK, f)
}

// --- List accepted friends ---

func (h *FriendsHandler) List(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header required"})
		return
	}

	rows, err := h.DB.QueryContext(c.Request.Context(),
		`SELECT u.id, u.name, u.email, u.avatar_url
		 FROM users u
		 JOIN friendships f ON (
		     (f.requester_id = $1 AND f.receiver_id = u.id) OR
		     (f.receiver_id  = $1 AND f.requester_id = u.id)
		 )
		 WHERE f.status = 'accepted'`,
		userID,
	)
	if err != nil {
		log.Printf("list friends for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer rows.Close()

	friends := []gin.H{}
	for rows.Next() {
		var id, name, email string
		var avatarURL sql.NullString
		if err := rows.Scan(&id, &name, &email, &avatarURL); err != nil {
			log.Printf("list friends scan: %v", err)
			continue
		}
		friends = append(friends, gin.H{
			"id":         id,
			"name":       name,
			"email":      email,
			"avatar_url": avatarURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

// --- Pending incoming invites ---

func (h *FriendsHandler) Pending(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header required"})
		return
	}

	rows, err := h.DB.QueryContext(c.Request.Context(),
		`SELECT f.id, f.requester_id, f.status, f.created_at,
		        u.name, u.email, u.avatar_url
		 FROM friendships f
		 JOIN users u ON u.id = f.requester_id
		 WHERE f.receiver_id = $1 AND f.status = 'pending'`,
		userID,
	)
	if err != nil {
		log.Printf("pending invites for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer rows.Close()

	invites := []gin.H{}
	for rows.Next() {
		var fID, requesterID, status string
		var createdAt time.Time
		var name, email string
		var avatarURL sql.NullString
		if err := rows.Scan(&fID, &requesterID, &status, &createdAt, &name, &email, &avatarURL); err != nil {
			log.Printf("pending invites scan: %v", err)
			continue
		}
		invites = append(invites, gin.H{
			"friendship_id": fID,
			"status":        status,
			"created_at":    createdAt,
			"sender": gin.H{
				"id":         requesterID,
				"name":       name,
				"email":      email,
				"avatar_url": avatarURL,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"pending": invites})
}
