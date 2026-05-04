package handlers

import (
	"database/sql"
	"log"
	"net/http"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"

	"github.com/xero7412/pick-me/backend/models"
)

type AuthHandler struct {
	DB           *sql.DB
	FirebaseAuth *firebaseauth.Client
}

func NewAuthHandler(db *sql.DB, firebaseAuth *firebaseauth.Client) *AuthHandler {
	return &AuthHandler{DB: db, FirebaseAuth: firebaseAuth}
}

type loginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	if h.FirebaseAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Firebase not configured"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	token, err := h.FirebaseAuth.VerifyIDToken(c.Request.Context(), req.IDToken)
	if err != nil {
		log.Printf("firebase token verification failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	googleID := token.UID
	name, _ := token.Claims["name"].(string)
	email, _ := token.Claims["email"].(string)
	picture, _ := token.Claims["picture"].(string)

	user, err := upsertUser(h.DB, googleID, name, email, picture)
	if err != nil {
		log.Printf("upsertUser failed for google_id=%s: %v", googleID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetUserFromToken looks up the DB user whose google_id matches the uid set by auth middleware.
func GetUserFromToken(db *sql.DB, c *gin.Context) (*models.User, error) {
	googleID := c.GetString("uid")
	var user models.User
	err := db.QueryRowContext(c.Request.Context(),
		`SELECT id, google_id, name, email, avatar_url, created_at
		 FROM users WHERE google_id = $1`,
		googleID,
	).Scan(&user.ID, &user.GoogleID, &user.Name, &user.Email, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// upsertUser inserts a new user or updates name/email/avatar on conflict, then returns the row.
func upsertUser(db *sql.DB, googleID, name, email, avatarURL string) (*models.User, error) {
	var user models.User
	err := db.QueryRow(
		`INSERT INTO users (google_id, name, email, avatar_url)
		 VALUES ($1, $2, $3, NULLIF($4, ''))
		 ON CONFLICT (google_id) DO UPDATE
		   SET name       = EXCLUDED.name,
		       email      = EXCLUDED.email,
		       avatar_url = EXCLUDED.avatar_url
		 RETURNING id, google_id, name, email, avatar_url, created_at`,
		googleID, name, email, avatarURL,
	).Scan(&user.ID, &user.GoogleID, &user.Name, &user.Email, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
