package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/leosu-cela/dog-company/pkg/tool"
)

const (
	HeaderAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
	ContextKeyUID       = "uid"
)

// AuthOptional parses a Bearer token if present. When the token is absent
// or invalid, the request proceeds anonymously (no UID in context).
// Handlers downstream MUST treat UID as optional via UIDFromContext.
func AuthOptional(verifier Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearerToken(c.GetHeader(HeaderAuthorization))
		if raw == "" {
			c.Next()
			return
		}
		claims, err := verifier.Verify(raw)
		if err != nil {
			c.Next()
			return
		}
		c.Set(ContextKeyUID, claims.UID)
		c.Next()
	}
}

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
		c.Set(ContextKeyUID, claims.UID)
		c.Next()
	}
}

func UIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextKeyUID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

func extractBearerToken(header string) string {
	if !strings.HasPrefix(header, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
}
