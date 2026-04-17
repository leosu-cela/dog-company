package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	ErrRefreshNotFound = errors.New("refresh token not found")
	ErrRefreshRevoked  = errors.New("refresh token already used or revoked")
	ErrRefreshExpired  = errors.New("refresh token expired")
)

type IRefreshTokenRepository interface {
	Create(tx *gorm.DB, rt *RefreshToken) error
	FindByHash(tx *gorm.DB, hash string) (*RefreshToken, error)
	Revoke(tx *gorm.DB, id uint64, replacedByHash *string) error
	RevokeAllByUser(tx *gorm.DB, userID uint64) error
}

type RefreshTokenRepository struct{}

func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{}
}

func (rep *RefreshTokenRepository) Create(tx *gorm.DB, rt *RefreshToken) error {
	if err := tx.Create(rt).Error; err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (rep *RefreshTokenRepository) FindByHash(tx *gorm.DB, hash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := tx.Where("token_hash = ?", hash).First(&rt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefreshNotFound
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return &rt, nil
}

func (rep *RefreshTokenRepository) Revoke(tx *gorm.DB, id uint64, replacedByHash *string) error {
	now := time.Now()
	updates := map[string]interface{}{"revoked_at": &now}
	if replacedByHash != nil {
		updates["replaced_by_hash"] = *replacedByHash
	}
	if err := tx.Model(&RefreshToken{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (rep *RefreshTokenRepository) RevokeAllByUser(tx *gorm.DB, userID uint64) error {
	now := time.Now()
	err := tx.Model(&RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": &now}).Error
	if err != nil {
		return fmt.Errorf("revoke all user refresh tokens: %w", err)
	}
	return nil
}

// GenerateRefreshToken returns a cryptographically random token and its SHA-256 hex hash.
// The raw token is returned to the client; only the hash is stored in the DB.
func GenerateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = HashRefreshToken(raw)
	return raw, hash, nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
