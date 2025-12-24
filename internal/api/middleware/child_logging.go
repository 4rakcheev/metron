package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// responseBodyWriter wraps gin.ResponseWriter to capture response body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// ChildAPILogging logs Child API requests and responses
func ChildAPILogging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only log /child/* routes
		if !strings.HasPrefix(c.Request.URL.Path, "/child") {
			c.Next()
			return
		}

		start := time.Now()

		// Capture request body
		var requestBody interface{}
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore the body for handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Try to parse as JSON
				if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
					// If not JSON, store as string
					requestBody = string(bodyBytes)
				}
			}
		}

		// Capture response body
		blw := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = blw

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Parse response body
		var responseBody interface{}
		if blw.body.Len() > 0 {
			if err := json.Unmarshal(blw.body.Bytes(), &responseBody); err != nil {
				// If not JSON, store as string
				responseBody = blw.body.String()
			}
		}

		// Sanitize sensitive data from request
		requestBodySanitized := sanitizeData(requestBody)

		// Log the request/response
		logAttrs := []slog.Attr{
			slog.String("request_id", c.GetString(RequestIDKey)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("query", c.Request.URL.RawQuery),
			slog.Int("status", c.Writer.Status()),
			slog.String("duration", duration.String()),
			slog.String("client_ip", c.ClientIP()),
		}

		if requestBodySanitized != nil {
			logAttrs = append(logAttrs, slog.Any("request_body", requestBodySanitized))
		}

		if responseBody != nil {
			logAttrs = append(logAttrs, slog.Any("response_body", responseBody))
		}

		// Get child_id from context if available (set by auth middleware)
		if childID, exists := c.Get("child_id"); exists {
			logAttrs = append(logAttrs, slog.String("child_id", childID.(string)))
		}

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "Child API request", logAttrs...)
	}
}

// sanitizeData removes sensitive fields from logged data
func sanitizeData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	// If it's a map, sanitize sensitive keys
	if m, ok := data.(map[string]interface{}); ok {
		sanitized := make(map[string]interface{})
		for k, v := range m {
			// Sanitize sensitive fields
			if k == "pin" || k == "password" || k == "token" || k == "secret" {
				sanitized[k] = "***REDACTED***"
			} else {
				sanitized[k] = v
			}
		}
		return sanitized
	}

	return data
}
