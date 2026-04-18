package tool

import "github.com/gin-gonic/gin"

const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 4010
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 5000

	// Shared spec codes (saves / leaderboard / future features).
	CodeBadPayload   = 4000
	CodeSanityFailed = 4002

	// Save-specific codes (docs/saves-api.md spec).
	CodeSaveUnsupportedVersion = 4001
	CodeSaveConflict           = 4090
)

type CommonResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(data interface{}) CommonResponse {
	return CommonResponse{Code: CodeOK, Data: data}
}

func Err(code int, message string) CommonResponse {
	return CommonResponse{Code: code, Message: message}
}

func ErrWithData(code int, message string, data interface{}) CommonResponse {
	return CommonResponse{Code: code, Message: message, Data: data}
}

var codeToStatus = map[int]int{
	CodeOK:                     200,
	CodeBadRequest:             400,
	CodeBadPayload:             400,
	CodeSanityFailed:           400,
	CodeSaveUnsupportedVersion: 400,
	CodeUnauthorized:           401,
	CodeForbidden:              403,
	CodeNotFound:               404,
	CodeConflict:               409,
	CodeSaveConflict:           409,
	CodeInternal:               500,
}

func WriteByHeader(c *gin.Context, res *CommonResponse) {
	status, ok := codeToStatus[res.Code]
	if !ok {
		status = 500
	}
	c.JSON(status, res)
}
