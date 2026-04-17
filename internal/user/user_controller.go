package user

import (
	"github.com/gin-gonic/gin"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

type UserController struct {
	handler *UserHandler
}

func NewUserController(handler *UserHandler) *UserController {
	return &UserController{handler: handler}
}

type registerRequest struct {
	Account  string `json:"account"  binding:"required" example:"leo@cela-tech.com"`
	Password string `json:"password" binding:"required" example:"hunter2000"`
}

type loginRequest struct {
	Account  string `json:"account"  binding:"required" example:"leo@cela-tech.com"`
	Password string `json:"password" binding:"required" example:"hunter2000"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"abc123..."`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"abc123..."`
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Create a new account. Account is normalized to lowercase; accepts email or [a-z0-9_]{3,30}. Password must be 8-72 chars.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerRequest		true	"register payload"
//	@Success		200		{object}	tool.CommonResponse{data=RegisterOutput}
//	@Failure		400		{object}	tool.CommonResponse	"invalid request body / validation failed"
//	@Failure		409		{object}	tool.CommonResponse	"account already exists"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/auth/register [post]
func (ctrl *UserController) Register(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		res = tool.Err(tool.CodeBadRequest, "invalid request body")
		return
	}

	data, commonRes := ctrl.handler.Register(c.Request.Context(), RegisterInput{
		Account:  req.Account,
		Password: req.Password,
	})
	if commonRes.Code != tool.CodeOK {
		res = commonRes
		return
	}
	res = tool.OK(data)
}

// Login godoc
//
//	@Summary		Login with account + password
//	@Description	Returns an access token (6h) and a refresh token (30d). Use the access token in Authorization header; use the refresh token at /auth/refresh to rotate.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest		true	"login payload"
//	@Success		200		{object}	tool.CommonResponse{data=LoginOutput}
//	@Failure		400		{object}	tool.CommonResponse	"invalid request body"
//	@Failure		401		{object}	tool.CommonResponse	"account or password incorrect"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/auth/login [post]
func (ctrl *UserController) Login(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		res = tool.Err(tool.CodeBadRequest, "invalid request body")
		return
	}

	data, commonRes := ctrl.handler.Login(c.Request.Context(), LoginInput{
		Account:  req.Account,
		Password: req.Password,
	})
	if commonRes.Code != tool.CodeOK {
		res = commonRes
		return
	}
	res = tool.OK(data)
}

// Refresh godoc
//
//	@Summary		Rotate refresh token and issue new access token
//	@Description	Exchange a valid refresh token for a fresh access + refresh pair. The old refresh token is revoked (one-time use). If a revoked token is reused, all of the user's refresh tokens are invalidated (reuse detection) and the client must login again.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		refreshRequest		true	"refresh payload"
//	@Success		200		{object}	tool.CommonResponse{data=RefreshOutput}
//	@Failure		400		{object}	tool.CommonResponse	"refresh_token required"
//	@Failure		401		{object}	tool.CommonResponse	"invalid / expired / reused refresh token"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/auth/refresh [post]
func (ctrl *UserController) Refresh(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		res = tool.Err(tool.CodeBadRequest, "invalid request body")
		return
	}

	data, commonRes := ctrl.handler.Refresh(c.Request.Context(), RefreshInput{
		RefreshToken: req.RefreshToken,
	})
	if commonRes.Code != tool.CodeOK {
		res = commonRes
		return
	}
	res = tool.OK(data)
}

// Logout godoc
//
//	@Summary		Revoke a refresh token
//	@Description	Invalidate the given refresh token. Idempotent — unknown or already-revoked tokens return OK. Access tokens remain valid until their 6h TTL; this endpoint does not revoke them.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		logoutRequest		true	"logout payload"
//	@Success		200		{object}	tool.CommonResponse
//	@Failure		400		{object}	tool.CommonResponse	"invalid request body"
//	@Failure		500		{object}	tool.CommonResponse	"internal error"
//	@Router			/auth/logout [post]
func (ctrl *UserController) Logout(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		res = tool.Err(tool.CodeBadRequest, "invalid request body")
		return
	}

	res = ctrl.handler.Logout(c.Request.Context(), LogoutInput{
		RefreshToken: req.RefreshToken,
	})
}

// Me godoc
//
//	@Summary		Get current user
//	@Description	Returns the user identified by the JWT in the Authorization header.
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	tool.CommonResponse{data=MeOutput}
//	@Failure		401	{object}	tool.CommonResponse	"missing / invalid / expired token"
//	@Failure		500	{object}	tool.CommonResponse	"internal error"
//	@Router			/auth/me [get]
func (ctrl *UserController) Me(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	uid, ok := auth.UIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	data, commonRes := ctrl.handler.Me(c.Request.Context(), uid)
	if commonRes.Code != tool.CodeOK {
		res = commonRes
		return
	}
	res = tool.OK(data)
}
