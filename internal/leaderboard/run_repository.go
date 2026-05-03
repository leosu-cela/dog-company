package leaderboard

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrRunNotFound = errors.New("run not found")

type IRunRepository interface {
	Upsert(tx *gorm.DB, userID uint64, goal int) (*Run, error)
	FindByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Run, error)
	FindByUserAndGoalForUpdate(tx *gorm.DB, userID uint64, goal int) (*Run, error)
	DeleteByUserAndGoal(tx *gorm.DB, userID uint64, goal int) error
}

type RunRepository struct{}

func NewRunRepository() *RunRepository {
	return &RunRepository{}
}

// Upsert 開新局：覆蓋同 (user_id, goal) 的 started_at 為現在。
// 同時回傳寫入後的 row，供呼叫端使用。
func (rep *RunRepository) Upsert(tx *gorm.DB, userID uint64, goal int) (*Run, error) {
	now := time.Now()
	r := &Run{
		UserID:    userID,
		Goal:      goal,
		StartedAt: now,
		UpdatedAt: now,
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "goal"}},
		DoUpdates: clause.Assignments(map[string]any{
			"started_at": now,
			"updated_at": now,
		}),
	}).Create(r).Error
	if err != nil {
		return nil, fmt.Errorf("upsert run: %w", err)
	}
	return r, nil
}

func (rep *RunRepository) FindByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Run, error) {
	var r Run
	err := tx.Where("user_id = ? AND goal = ?", userID, goal).First(&r).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("find run: %w", err)
	}
	return &r, nil
}

func (rep *RunRepository) FindByUserAndGoalForUpdate(tx *gorm.DB, userID uint64, goal int) (*Run, error) {
	var r Run
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND goal = ?", userID, goal).
		First(&r).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("find run for update: %w", err)
	}
	return &r, nil
}

func (rep *RunRepository) DeleteByUserAndGoal(tx *gorm.DB, userID uint64, goal int) error {
	err := tx.Where("user_id = ? AND goal = ?", userID, goal).Delete(&Run{}).Error
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	return nil
}
