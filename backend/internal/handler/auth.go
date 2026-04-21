package handler

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type verifyRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code"  binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register stores a pending verification and emails a 6-digit code.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reject if email already registered
	var exists int
	h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = $1`, req.Email).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}

	code := fmt.Sprintf("%06d", rand.Intn(1_000_000))
	expires := time.Now().Add(15 * time.Minute)

	// Upsert pending verification (allow resend)
	_, err = h.db.Exec(`
		INSERT INTO pending_verifications (email, password_hash, code, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET password_hash = EXCLUDED.password_hash,
		      code = EXCLUDED.code,
		      expires_at = EXCLUDED.expires_at
	`, req.Email, string(hash), code, expires)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := sendVerificationEmail(req.Email, code); err != nil {
		// Remove pending record so user can retry
		h.db.Exec(`DELETE FROM pending_verifications WHERE email = $1`, req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pending": true, "email": req.Email})
}

// Verify confirms the code and creates the user account.
func (h *Handler) Verify(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var hash, code string
	var expiresAt time.Time
	err := h.db.QueryRow(`
		SELECT password_hash, code, expires_at FROM pending_verifications WHERE email = $1
	`, req.Email).Scan(&hash, &code, &expiresAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no pending registration for this email"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if time.Now().After(expiresAt) {
		h.db.Exec(`DELETE FROM pending_verifications WHERE email = $1`, req.Email)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "code expired, please register again"})
		return
	}
	if req.Code != code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect code"})
		return
	}

	// Create the user
	id := uuid.New().String()
	_, err = h.db.Exec(
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`,
		id, req.Email, hash,
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	h.db.Exec(`DELETE FROM pending_verifications WHERE email = $1`, req.Email)

	token, err := generateToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token, "user_id": id, "email": req.Email})
}

// Login authenticates an existing verified user.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id, hash string
	err := h.db.QueryRow(
		`SELECT id, password_hash FROM users WHERE email = $1`, req.Email,
	).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := generateToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": id, "email": req.Email})
}

func generateToken(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret()))
}

func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "changeme-in-production"
}
