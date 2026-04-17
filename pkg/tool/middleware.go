package tool

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodySize wraps c.Request.Body so reads beyond maxBytes fail.
// When a handler calls c.ShouldBindJSON on an oversized body, the JSON
// decoder surfaces an "http: request body too large" error, which the
// handler turns into a 400.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
