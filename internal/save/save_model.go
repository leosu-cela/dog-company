package save

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Save struct {
	UserUID   uuid.UUID       `gorm:"column:user_uid;primaryKey;type:uuid"`
	Version   int             `gorm:"column:version;not null"`
	Revision  int             `gorm:"column:revision;not null;default:1"`
	Data      json.RawMessage `gorm:"column:data;type:jsonb;not null"`
	UpdatedAt time.Time       `gorm:"column:updated_at;not null;default:now()"`
}

func (Save) TableName() string { return "saves" }
