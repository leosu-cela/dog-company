package events

import (
	"fmt"

	"gorm.io/gorm"
)

type IEventRepository interface {
	BulkInsert(tx *gorm.DB, events []Event) error
}

type EventRepository struct{}

func NewEventRepository() *EventRepository {
	return &EventRepository{}
}

// BulkInsert 一次寫入多筆。空 slice 直接 noop。
// 使用 gorm 的 batch insert（CreateInBatches 預設批次太細，這裡單次 INSERT）。
func (rep *EventRepository) BulkInsert(tx *gorm.DB, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	if err := tx.Create(&events).Error; err != nil {
		return fmt.Errorf("bulk insert events: %w", err)
	}
	return nil
}
