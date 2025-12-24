package bot

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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

// BotLoggingMiddleware logs bot webhook requests and responses
func BotLoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Capture request body
		var requestBody interface{}
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore the body for handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Try to parse as JSON (Telegram updates are JSON)
				if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
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
				responseBody = blw.body.String()
			}
		}

		// Extract useful information from Telegram update
		var chatID interface{}
		var userName string
		var updateType string
		var commandOrCallback string

		if req, ok := requestBody.(map[string]interface{}); ok {
			// Extract message info
			if msg, ok := req["message"].(map[string]interface{}); ok {
				updateType = "message"
				if chat, ok := msg["chat"].(map[string]interface{}); ok {
					chatID = chat["id"]
				}
				if from, ok := msg["from"].(map[string]interface{}); ok {
					if username, ok := from["username"].(string); ok {
						userName = username
					}
				}
				if text, ok := msg["text"].(string); ok {
					commandOrCallback = text
				}
			}

			// Extract callback query info
			if callback, ok := req["callback_query"].(map[string]interface{}); ok {
				updateType = "callback_query"
				if msg, ok := callback["message"].(map[string]interface{}); ok {
					if chat, ok := msg["chat"].(map[string]interface{}); ok {
						chatID = chat["id"]
					}
				}
				if from, ok := callback["from"].(map[string]interface{}); ok {
					if username, ok := from["username"].(string); ok {
						userName = username
					}
				}
				if data, ok := callback["data"].(string); ok {
					commandOrCallback = data
				}
			}
		}

		// Build log attributes
		logAttrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.String("duration", duration.String()),
			slog.String("client_ip", c.ClientIP()),
		}

		if chatID != nil {
			logAttrs = append(logAttrs, slog.Any("chat_id", chatID))
		}
		if userName != "" {
			logAttrs = append(logAttrs, slog.String("username", userName))
		}
		if updateType != "" {
			logAttrs = append(logAttrs, slog.String("update_type", updateType))
		}
		if commandOrCallback != "" {
			logAttrs = append(logAttrs, slog.String("command_or_callback", commandOrCallback))
		}

		// Include full request/response for debugging
		if requestBody != nil {
			logAttrs = append(logAttrs, slog.Any("request", requestBody))
		}
		if responseBody != nil {
			logAttrs = append(logAttrs, slog.Any("response", responseBody))
		}

		// Log errors at error level, everything else at info
		if len(c.Errors) > 0 {
			logAttrs = append(logAttrs, slog.String("errors", c.Errors.String()))
			logger.LogAttrs(c.Request.Context(), slog.LevelError, "Bot webhook request", logAttrs...)
		} else {
			logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "Bot webhook request", logAttrs...)
		}
	}
}
