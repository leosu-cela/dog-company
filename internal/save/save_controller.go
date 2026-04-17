package save

import (
	"github.com/gin-gonic/gin"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

type SaveController struct {
	handler *SaveHandler
}

func NewSaveController(handler *SaveHandler) *SaveController {
	return &SaveController{handler: handler}
}

// Get godoc
//
//	@Summary		Get current save
//	@Description	Returns the authenticated user's current save. If no save exists, data is null.
//	@Tags			saves
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	tool.CommonResponse{data=GetOutput}
//	@Failure		401	{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Failure		500	{object}	tool.CommonResponse	"internal error"
//	@Router			/saves [get]
func (ctrl *SaveController) Get(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	res = ctrl.handler.Get(c.Request.Context(), uid)
}

// Upsert godoc
//
//	@Summary		Create or overwrite current save
//	@Description	Creates the save if absent (accepts any client revision, server starts at 1). If present, client revision must equal server's current revision, otherwise 409 with server state. Body is capped at 64 KiB.
//	@Tags			saves
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		SavePayload	true	"save payload"
//	@Success		200		{object}	tool.CommonResponse{data=UpsertOutput}
//	@Failure		400		{object}	tool.CommonResponse	"bad payload / unsupported version / sanity failed"
//	@Failure		401		{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Failure		409		{object}	tool.CommonResponse{data=ConflictData}	"revision conflict; data carries server state"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/saves [post]
func (ctrl *SaveController) Upsert(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	var payload SavePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		res = tool.Err(tool.CodeSaveBadPayload, "invalid request body")
		return
	}

	res = ctrl.handler.Upsert(c.Request.Context(), uid, payload)
}

// Delete godoc
//
//	@Summary		Delete current save
//	@Description	Idempotently deletes the authenticated user's save. Returns 200 even when no save existed.
//	@Tags			saves
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	tool.CommonResponse
//	@Failure		401	{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Failure		500	{object}	tool.CommonResponse	"internal error"
//	@Router			/saves [delete]
func (ctrl *SaveController) Delete(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	res = ctrl.handler.Delete(c.Request.Context(), uid)
}
