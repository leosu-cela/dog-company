package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/leosu-cela/dog-company/pkg/tool"
)

const (
	HeaderAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
	ContextKeyUserID    = "user_id"
)

func AuthRequired(verifier Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearerToken(c.GetHeader(HeaderAuthorization))
		if raw == "" {
			res := tool.Err(tool.CodeUnauthorized, "missing or malformed token")
			tool.WriteByHeader(c, &res)
			c.Abort()
			return
		}
		claims, err := verifier.Verify(raw)
		if err != nil {
			res := tool.Err(tool.CodeUnauthorized, "invalid or expired token")
			tool.WriteByHeader(c, &res)
			c.Abort()
			return
		}
		c.Set(ContextKeyUserID, claims.UserID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uint64, bool) {
	v, ok := c.Get(ContextKeyUserID)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint64)
	return id, ok
}

func extractBearerToken(header string) string {
	if !strings.HasPrefix(header, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
}
