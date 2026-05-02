package leaderboard

import "time"

type Entry struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement"`
	UserID            uint64    `gorm:"column:user_id;not null"`
	Nickname          string    `gorm:"column:nickname;not null;size:64"`     // 帳號（保留欄位；前端不再顯示）
	CompanyName       string    `gorm:"column:company_name;size:32"`           // 玩家自訂公司名（v6 起；nullable，舊資料為空）
	Days              int       `gorm:"column:days;not null"`
	Money             int       `gorm:"column:money;not null"`
	Goal              int       `gorm:"column:goal;not null"`
	OfficeLevel       int       `gorm:"column:office_level;not null"`
	StaffCount        int       `gorm:"column:staff_count;not null"`
	ProjectsCompleted int       `gorm:"column:projects_completed;not null;default:0"`
	SubmittedAt       time.Time `gorm:"column:submitted_at;not null;default:now()"`
}

func (Entry) TableName() string { return "leaderboard_entries" }
