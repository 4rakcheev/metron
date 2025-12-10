package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ContentType enforces JSON content-type for POST and PATCH requests
func ContentType() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPatch {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Content-Type must be application/json",
					"code":  "INVALID_CONTENT_TYPE",
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
