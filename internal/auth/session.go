package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"gorm.io/gorm"
)

const SessionDuration = 30 * 24 * time.Hour // 30 days

type SessionService struct {
	DB *database.DB
}

func NewSessionService(db *database.DB) *SessionService {
	return &SessionService{DB: db}
}

// GenerateRandomToken generates a cryptographically secure random token.
func GenerateRandomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of a token.
func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// CreateSession generates a new session token, saves its hash, and returns the raw token.
func (s *SessionService) CreateSession(userID uint) (string, *models.Session, error) {
	rawToken, err := GenerateRandomToken(32)
	if err != nil {
		return "", nil, err
	}

	tokenHash := HashToken(rawToken)
	now := time.Now().UTC()
	sess := &models.Session{
		ID:        tokenHash,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	if err := s.DB.Create(sess).Error; err != nil {
		return "", nil, fmt.Errorf("failed to create session: %w", err)
	}

	return rawToken, sess, nil
}

// ValidateSession looks up the session by raw token and verifies it is active.
func (s *SessionService) ValidateSession(rawToken string) (*models.User, *models.Session, error) {
	if rawToken == "" {
		return nil, nil, gorm.ErrRecordNotFound
	}

	tokenHash := HashToken(rawToken)
	var sess models.Session
	if err := s.DB.Where("id = ?", tokenHash).First(&sess).Error; err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	if sess.RevokedAt != nil || sess.ExpiresAt.Before(now) {
		return nil, nil, gorm.ErrRecordNotFound
	}

	var user models.User
	if err := s.DB.First(&user, sess.UserID).Error; err != nil {
		return nil, nil, err
	}

	if user.Status != "active" {
		return nil, nil, fmt.Errorf("user account is %s", user.Status)
	}

	return &user, &sess, nil
}

// RevokeSession marks a session as revoked.
func (s *SessionService) RevokeSession(rawToken string) error {
	if rawToken == "" {
		return nil
	}
	tokenHash := HashToken(rawToken)
	now := time.Now().UTC()
	return s.DB.Model(&models.Session{}).Where("id = ?", tokenHash).Update("revoked_at", &now).Error
}
