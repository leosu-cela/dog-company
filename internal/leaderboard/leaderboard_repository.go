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
	ListByUser(tx *gorm.DB, userID uint64, goal, limit int) ([]Entry, error)
	FindBestByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Entry, error)
	FindRecentDuplicate(tx *gorm.DB, userID uint64, goal, days, money int, within time.Duration) (*Entry, error)
	CountBetter(tx *gorm.DB, goal, days, money int) (int64, error)
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
		Order("days ASC, money DESC, id ASC").
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	return entries, nil
}

// ListByUser returns the user's entries. If goal > 0, filter by goal; else all goals.
// Ordered by submitted_at DESC (most recent first).
func (rep *EntryRepository) ListByUser(tx *gorm.DB, userID uint64, goal, limit int) ([]Entry, error) {
	q := tx.Where("user_id = ?", userID)
	if goal > 0 {
		q = q.Where("goal = ?", goal)
	}
	var entries []Entry
	err := q.Order("submitted_at DESC, id DESC").Limit(limit).Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("list by user: %w", err)
	}
	return entries, nil
}

// FindBestByUserAndGoal returns the user's best entry for the given goal
// (lowest days, then highest money, then earliest id).
func (rep *EntryRepository) FindBestByUserAndGoal(tx *gorm.DB, userID uint64, goal int) (*Entry, error) {
	var e Entry
	err := tx.Where("user_id = ? AND goal = ?", userID, goal).
		Order("days ASC, money DESC, id ASC").
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find best: %w", err)
	}
	return &e, nil
}

func (rep *EntryRepository) FindRecentDuplicate(tx *gorm.DB, userID uint64, goal, days, money int, within time.Duration) (*Entry, error) {
	var e Entry
	cutoff := time.Now().Add(-within)
	err := tx.Where("user_id = ? AND goal = ? AND days = ? AND money = ? AND submitted_at > ?",
		userID, goal, days, money, cutoff).
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

// CountBetter returns the number of entries that rank strictly above (goal, days, money):
// fewer days, or same days but more money.
func (rep *EntryRepository) CountBetter(tx *gorm.DB, goal, days, money int) (int64, error) {
	var count int64
	err := tx.Model(&Entry{}).
		Where("goal = ? AND (days < ? OR (days = ? AND money > ?))", goal, days, days, money).
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
