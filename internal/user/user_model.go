package user

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Account      string    `gorm:"column:account;not null;size:255"`
	PasswordHash string    `gorm:"column:password_hash;not null;size:255"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;default:now()"`
}

func (User) TableName() string { return "users" }
