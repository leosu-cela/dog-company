package leaderboard

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("entry not found")

type IEntryRepository interface {
	Create(tx *gorm.DB, e *Entry) error
	List(tx *gorm.DB, goal, limit int) ([]Entry, error)
	FindBestByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Entry, error)
	FindRecentDuplicate(tx *gorm.DB, userID uint64, goal, days, money, projectsCompleted int, within time.Duration) (*Entry, error)
	CountBetter(tx *gorm.DB, goal, days, money, projectsCompleted int) (int64, error)
	CountByGoal(tx *gorm.DB, goal int) (int64, error)
}

type EntryRepository struct{}

func NewEntryRepository() *EntryRepository {
	return &EntryRepository{}
}

func (rep *EntryRepository) Create(tx *gorm.DB, e *Entry) error {
	if err := tx.Create(e).Error; err != nil {
		return fmt.Errorf("create entry: %w", err)
	}
	return nil
}

func (rep *EntryRepository) List(tx *gorm.DB, goal, limit int) ([]Entry, error) {
	var entries []Entry
	err := tx.Where("goal = ?", goal).
		Order("days ASC, money DESC, projects_completed DESC, id ASC").
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	return entries, nil
}

// FindBestByUserAndGoal returns the user's best entry for the given goal
// per the global sort key (days ASC, money DESC, projects_completed DESC, id ASC).
func (rep *EntryRepository) FindBestByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Entry, error) {
	var e Entry
	err := tx.Where("user_id = ? AND goal = ?", userID, goal).
		Order("days ASC, money DESC, projects_completed DESC, id ASC").
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find best: %w", err)
	}
	return &e, nil
}

func (rep *EntryRepository) FindRecentDuplicate(tx *gorm.DB, userID uint64, goal, days, money, projectsCompleted int, within time.Duration) (*Entry, error) {
	var e Entry
	cutoff := time.Now().Add(-within)
	err := tx.Where(
		"user_id = ? AND goal = ? AND days = ? AND money = ? AND projects_completed = ? AND submitted_at > ?",
		userID, goal, days, money, projectsCompleted, cutoff,
	).
		Order("id DESC").
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find recent duplicate: %w", err)
	}
	return &e, nil
}

// CountBetter returns the number of entries that rank strictly above
// (goal, days, money, projects_completed) per the global sort key.
func (rep *EntryRepository) CountBetter(tx *gorm.DB, goal, days, money, projectsCompleted int) (int64, error) {
	var count int64
	err := tx.Model(&Entry{}).
		Where(
			"goal = ? AND ("+
				"days < ? "+
				"OR (days = ? AND money > ?) "+
				"OR (days = ? AND money = ? AND projects_completed > ?)"+
				")",
			goal,
			days,
			days, money,
			days, money, projectsCompleted,
		).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count better: %w", err)
	}
	return count, nil
}

func (rep *EntryRepository) CountByGoal(tx *gorm.DB, goal int) (int64, error) {
	var count int64
	err := tx.Model(&Entry{}).Where("goal = ?", goal).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count by goal: %w", err)
	}
	return count, nil
}
