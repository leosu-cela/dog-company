package auth

import "time"

type RefreshToken struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	UserID         uint64     `gorm:"column:user_id;not null"`
	TokenHash      string     `gorm:"column:token_hash;not null;size:64"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt      *time.Time `gorm:"column:revoked_at"`
	ReplacedByHash *string    `gorm:"column:replaced_by_hash;size:64"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:now()"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
