package events

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/leosu-cela/dog-company/pkg/tool"
)

type EventHandler struct {
	buffer *Buffer
}

func NewEventHandler(buffer *Buffer) *EventHandler {
	return &EventHandler{buffer: buffer}
}

type LogPayload struct {
	Type    string          `json:"type"    binding:"required" example:"office_upgrade"`
	Payload json.RawMessage `json:"payload"                    swaggertype:"object"`
}

// Log 將事件加入緩衝。即時回應，實際 DB 寫入由 buffer 在達到 100 筆 / idle 5 分鐘 / 關機時批次處理。
// uid 來自 JWT，無需查 DB；FK 由 DB 在 flush 時驗。
func (handler *EventHandler) Log(ctx context.Context, uid uuid.UUID, payload LogPayload) tool.CommonResponse {
	if payload.Type == "" {
		return tool.Err(tool.CodeBadPayload, "event type is required")
	}
	if len(payload.Type) > 64 {
		return tool.Err(tool.CodeBadPayload, "event type too long (max 64)")
	}

	handler.buffer.Append(uid, payload.Type, payload.Payload)
	return tool.OK(nil)
}

// LogInternal 後端內部呼叫專用：吃 user.UID。payload 由 caller 自行 marshal，nil 代表無 payload。
func LogInternal(buffer *Buffer, userUID uuid.UUID, eventType string, payload any) {
	if buffer == nil {
		return
	}
	var raw json.RawMessage
	if payload != nil {
		bytes, err := json.Marshal(payload)
		if err == nil {
			raw = bytes
		}
	}
	buffer.Append(userUID, eventType, raw)
}
