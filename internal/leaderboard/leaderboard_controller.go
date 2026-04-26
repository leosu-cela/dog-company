package leaderboard

import (
	"github.com/gin-gonic/gin"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

type LeaderboardController struct {
	handler *LeaderboardHandler
}

func NewLeaderboardController(handler *LeaderboardHandler) *LeaderboardController {
	return &LeaderboardController{handler: handler}
}

type listQuery struct {
	Limit int `form:"limit" example:"10"`
	Goal  int `form:"goal"  example:"50000"`
}

// List godoc
//
//	@Summary		Get leaderboard top N (+ caller's me when authed)
//	@Description	Returns top entries sorted by days ASC, money DESC, projects_completed DESC. Auth is optional — if a valid Bearer token is provided and the user has any entry for this goal, the response also includes me with the user's best entry and global rank. Default goal=50000, default limit=10 (max 50).
//	@Tags			leaderboard
//	@Produce		json
//	@Param			limit	query		int	false	"max entries (default 10, max 50)"
//	@Param			goal	query		int	false	"goal amount (default 50000)"
//	@Success		200		{object}	tool.CommonResponse{data=ListOutput}
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/leaderboard [get]
func (ctrl *LeaderboardController) List(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	var q listQuery
	_ = c.ShouldBindQuery(&q)

	input := ListInput{Goal: q.Goal, Limit: q.Limit}
	if uid, ok := auth.UIDFromContext(c); ok {
		input.UID = &uid
	}

	res = ctrl.handler.List(c.Request.Context(), input)
}

// Submit godoc
//
//	@Summary		Submit a leaderboard entry
//	@Description	Records an IPO completion. Nickname is taken from the authenticated user's account (client cannot spoof). Duplicate submissions (same user+goal+days+money+projects_completed within 1 minute) are silently deduplicated and return the existing entry.
//	@Tags			leaderboard
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		SubmitPayload	true	"submit payload"
//	@Success		200		{object}	tool.CommonResponse{data=SubmitOutput}
//	@Failure		400		{object}	tool.CommonResponse	"bad payload / sanity failed"
//	@Failure		401		{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/leaderboard [post]
func (ctrl *LeaderboardController) Submit(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	var payload SubmitPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		res = tool.Err(tool.CodeBadPayload, "invalid request body")
		return
	}

	res = ctrl.handler.Submit(c.Request.Context(), uid, payload)
}
