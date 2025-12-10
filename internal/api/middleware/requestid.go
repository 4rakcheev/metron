package middleware

import (
	"metron/internal/idgen"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "X-Request-ID"

// RequestID injects a unique request ID into each request context
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			requestID = idgen.New()
		}
		c.Header(RequestIDKey, requestID)
		c.Set(RequestIDKey, requestID)
		c.Next()
	}
}
