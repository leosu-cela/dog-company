package user

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrNotFound   = errors.New("user not found")
	ErrDuplicated = errors.New("account already exists")
)

type IUserRepository interface {
	Create(tx *gorm.DB, u *User) error
	FindByAccount(tx *gorm.DB, account string) (*User, error)
	FindByID(tx *gorm.DB, id uint64) (*User, error)
	FindByUID(tx *gorm.DB, uid uuid.UUID) (*User, error)
}

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (rep *UserRepository) Create(tx *gorm.DB, u *User) error {
	if err := tx.Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicated
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (rep *UserRepository) FindByAccount(tx *gorm.DB, account string) (*User, error) {
	var u User
	err := tx.Where("account = ?", account).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by account: %w", err)
	}
	return &u, nil
}

func (rep *UserRepository) FindByID(tx *gorm.DB, id uint64) (*User, error) {
	var u User
	err := tx.First(&u, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}

func (rep *UserRepository) FindByUID(tx *gorm.DB, uid uuid.UUID) (*User, error) {
	var u User
	err := tx.Where("uid = ?", uid).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by uid: %w", err)
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}
