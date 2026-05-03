package events

import (
	"github.com/gin-gonic/gin"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

type EventController struct {
	handler *EventHandler
}

func NewEventController(handler *EventHandler) *EventController {
	return &EventController{handler: handler}
}

// Log godoc
//
//	@Summary		Log a player event
//	@Description	Records a gameplay event (office_upgrade, buy_shop_item, etc). Server fills user_id and timestamp. Events are buffered in memory and batch-flushed every 100 events / 5 minutes / on shutdown. Best-effort persistence; not for critical data.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		LogPayload	true	"event payload"
//	@Success		200		{object}	tool.CommonResponse
//	@Failure		400		{object}	tool.CommonResponse	"missing type"
//	@Failure		401		{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Router			/events [post]
func (ctrl *EventController) Log(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	var payload LogPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		res = tool.Err(tool.CodeBadPayload, "invalid request body")
		return
	}

	res = ctrl.handler.Log(c.Request.Context(), uid, payload)
}
