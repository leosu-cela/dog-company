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
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register godoc
// @Summary  Register a new user
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body  body  registerRequest  true  "register payload"
// @Success  200   {object}  tool.CommonResponse
// @Router   /api/v1/auth/register [post]
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
// @Summary  Login with account + password
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body  body  loginRequest  true  "login payload"
// @Success  200   {object}  tool.CommonResponse
// @Router   /api/v1/auth/login [post]
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

// Me godoc
// @Summary  Get current user
// @Tags     auth
// @Produce  json
// @Security BearerAuth
// @Success  200  {object}  tool.CommonResponse
// @Router   /api/v1/auth/me [get]
func (ctrl *UserController) Me(c *gin.Context) {
	var res tool.CommonResponse
	defer tool.WriteByHeader(c, &res)

	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		res = tool.Err(tool.CodeUnauthorized, "missing user context")
		return
	}

	data, commonRes := ctrl.handler.Me(c.Request.Context(), userID)
	if commonRes.Code != tool.CodeOK {
		res = commonRes
		return
	}
	res = tool.OK(data)
}
