package save

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("save not found")

type ISaveRepository interface {
	FindByUserUID(tx *gorm.DB, uid uuid.UUID) (*Save, error)
	FindByUserUIDForUpdate(tx *gorm.DB, uid uuid.UUID) (*Save, error)
	Create(tx *gorm.DB, s *Save) error
	UpdateRevisionAndData(tx *gorm.DB, uid uuid.UUID, version, newRevision int, data []byte, updatedAt time.Time) error
	DeleteByUserUID(tx *gorm.DB, uid uuid.UUID) error
}

type SaveRepository struct{}

func NewSaveRepository() *SaveRepository {
	return &SaveRepository{}
}

func (rep *SaveRepository) FindByUserUID(tx *gorm.DB, uid uuid.UUID) (*Save, error) {
	var s Save
	err := tx.Where("user_uid = ?", uid).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find save: %w", err)
	}
	return &s, nil
}

func (rep *SaveRepository) FindByUserUIDForUpdate(tx *gorm.DB, uid uuid.UUID) (*Save, error) {
	var s Save
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_uid = ?", uid).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find save for update: %w", err)
	}
	return &s, nil
}

func (rep *SaveRepository) Create(tx *gorm.DB, s *Save) error {
	if err := tx.Create(s).Error; err != nil {
		return fmt.Errorf("create save: %w", err)
	}
	return nil
}

func (rep *SaveRepository) UpdateRevisionAndData(tx *gorm.DB, uid uuid.UUID, version, newRevision int, data []byte, updatedAt time.Time) error {
	err := tx.Model(&Save{}).
		Where("user_uid = ?", uid).
		Updates(map[string]interface{}{
			"version":    version,
			"revision":   newRevision,
			"data":       data,
			"updated_at": updatedAt,
		}).Error
	if err != nil {
		return fmt.Errorf("update save: %w", err)
	}
	return nil
}

func (rep *SaveRepository) DeleteByUserUID(tx *gorm.DB, uid uuid.UUID) error {
	if err := tx.Where("user_uid = ?", uid).Delete(&Save{}).Error; err != nil {
		return fmt.Errorf("delete save: %w", err)
	}
	return nil
}
