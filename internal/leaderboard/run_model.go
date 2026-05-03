package leaderboard

import "time"

type Run struct {
	UserID    uint64    `gorm:"column:user_id;primaryKey"`
	Goal      int       `gorm:"column:goal;primaryKey"`
	StartedAt time.Time `gorm:"column:started_at;not null;default:now()"`
}

func (Run) TableName() string { return "leaderboard_runs" }
