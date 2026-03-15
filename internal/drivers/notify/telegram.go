package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpSender sends messages via the Telegram Bot API.
type httpSender struct {
	token  string
	client *http.Client
}

// newHTTPSender creates a new HTTP-based Telegram sender.
func newHTTPSender(token string) *httpSender {
	return &httpSender{
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// sendMessageRequest is the JSON body for Telegram sendMessage API.
type sendMessageRequest struct {
	ChatID      int64       `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

// SendMessage posts a message to a Telegram chat.
func (s *httpSender) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token)

	body := sendMessageRequest{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   "Markdown",
		ReplyMarkup: replyMarkup,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Rate limited — log-worthy but not fatal
		return fmt.Errorf("telegram rate limited (HTTP 429)")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
