package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// 預設支援的事件類型；前端送其他字串也會被收下，但建議走這份白名單以利分析。
const (
	TypeStartRun         = "start_run"
	TypeSubmitLeaderboard = "submit_leaderboard"
	TypeOfficeUpgrade    = "office_upgrade"
	TypeBuyShopItem      = "buy_shop_item"
)

type Event struct {
	ID        uint64          `gorm:"primaryKey;autoIncrement"`
	UserUID   uuid.UUID       `gorm:"column:user_uid;type:uuid;not null"`
	EventType string          `gorm:"column:event_type;not null;size:64"`
	Payload   json.RawMessage `gorm:"column:payload;type:jsonb"`
	CreatedAt time.Time       `gorm:"column:created_at;not null;default:now()"`
}

func (Event) TableName() string { return "event_logs" }
