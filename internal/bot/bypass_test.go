package bot

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestBuildBypassActionsButtons_NoIndefiniteOption(t *testing.T) {
	keyboard := BuildBypassActionsButtons("win-pc1", false)

	foundEndOfDay := false
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == nil {
				continue
			}
			data, err := UnmarshalCallback(*btn.CallbackData)
			if err != nil || data.Action != "bypass" || data.SubAction != "enable" {
				continue
			}
			if data.Duration == 0 {
				t.Errorf("button %q would enable an indefinite bypass", btn.Text)
			}
			if data.Duration == bypassUntilEndOfDay {
				foundEndOfDay = true
			}
		}
	}

	if !foundEndOfDay {
		t.Error("expected an 'Until end of day' option")
	}
}

func TestBuildBypassActionsButtons_EnabledShowsDisable(t *testing.T) {
	keyboard := BuildBypassActionsButtons("win-pc1", true)

	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == nil {
				continue
			}
			data, err := UnmarshalCallback(*btn.CallbackData)
			if err == nil && data.SubAction == "disable" && data.Device == "win-pc1" {
				return
			}
		}
	}
	t.Error("expected a Disable Bypass button when bypass is enabled")
}

func TestMinutesUntilEndOfDay(t *testing.T) {
	riga, err := time.LoadLocation("Europe/Riga")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	prev := timezone
	timezone = riga
	t.Cleanup(func() { timezone = prev })

	tests := []struct {
		name string
		now  time.Time
		want int
	}{
		{"evening", time.Date(2026, 9, 23, 19, 30, 0, 0, riga), 270},
		{"just before midnight", time.Date(2026, 9, 23, 23, 59, 30, 0, riga), 1},
		{"start of day", time.Date(2026, 9, 23, 0, 0, 0, 0, riga), 1440},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minutesUntilEndOfDay(tt.now.UTC()); got != tt.want {
				t.Errorf("minutesUntilEndOfDay() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	notFound := &StatusError{StatusCode: http.StatusNotFound, Message: "API error 404"}
	if !isNotFound(notFound) {
		t.Error("expected 404 StatusError to be not found")
	}
	if !isNotFound(fmt.Errorf("wrapped: %w", notFound)) {
		t.Error("expected wrapped 404 to be not found")
	}
	if isNotFound(&StatusError{StatusCode: http.StatusInternalServerError}) {
		t.Error("expected 500 not to be not found")
	}
	if isNotFound(errors.New("network down")) {
		t.Error("expected plain error not to be not found")
	}
}
