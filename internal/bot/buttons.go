package bot

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CallbackData represents the data embedded in callback buttons
type CallbackData struct {
	Action      string `json:"a"`            // Action type
	Step        int    `json:"s,omitempty"`  // Current step in flow
	ChildID     string `json:"c,omitempty"`  // Child ID (resolved from index)
	ChildIndex  int    `json:"ci,omitempty"` // Child index in list (for compact callback)
	Device      string `json:"d,omitempty"`  // Device type
	Duration    int    `json:"m,omitempty"`  // Duration in minutes
	Session     string `json:"ses,omitempty"` // Session ID (resolved from index)
	SessionIndex int   `json:"si,omitempty"` // Session index in list (for compact callback)
}

// MarshalCallback converts CallbackData to JSON string
func MarshalCallback(data CallbackData) string {
	b, err := json.Marshal(data)
	if err != nil {
		// Should never happen with simple structs
		return ""
	}
	return string(b)
}

// UnmarshalCallback parses callback data from JSON string
func UnmarshalCallback(data string) (*CallbackData, error) {
	var cb CallbackData
	if err := json.Unmarshal([]byte(data), &cb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal callback: %w", err)
	}
	return &cb, nil
}

// BuildChildrenButtons creates buttons for selecting children
func BuildChildrenButtons(children []Child, action string, step int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Individual children - use index instead of full UUID
	for i, child := range children {
		emoji := getChildEmoji(child.Name)
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: i, // Use index to keep callback data small
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, child.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// "Shared" option (all children) - use special index -1
	if len(children) > 1 {
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: -1, // Special marker for shared sessions
		})

		btn := tgbotapi.NewInlineKeyboardButtonData("👨‍👩‍👧 Shared (All)", callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildDevicesButtons creates buttons for selecting devices
func BuildDevicesButtons(devices []Device, action string, step int, childID string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, device := range devices {
		emoji := getDeviceEmoji(device.Type)
		callback := MarshalCallback(CallbackData{
			Action:  action,
			Step:    step,
			ChildID: childID,
			Device:  device.Type,
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, device.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Back button
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: action, Step: step - 1}),
	)

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildDurationButtons creates buttons for selecting duration
func BuildDurationButtons(action string, step int, childID, device string) tgbotapi.InlineKeyboardMarkup {
	durations := []int{5, 15, 30, 60, 120}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create two rows: [5, 15, 30] and [60, 120]
	row1 := []tgbotapi.InlineKeyboardButton{}
	row2 := []tgbotapi.InlineKeyboardButton{}

	for i, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:   action,
			Step:     step,
			ChildID:  childID,
			Device:   device,
			Duration: duration,
		})

		label := fmt.Sprintf("+%d", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)

		if i < 3 {
			row1 = append(row1, btn)
		} else {
			row2 = append(row2, btn)
		}
	}

	rows = append(rows, row1, row2)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: action, Step: step - 1, ChildID: childID}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildSessionsButtons creates buttons for selecting an active session
func BuildSessionsButtons(sessions []Session, action string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, session := range sessions {
		emoji := getDeviceEmoji(session.DeviceType)
		label := fmt.Sprintf("%d. %s %s", i+1, emoji, session.DeviceType)

		callback := MarshalCallback(CallbackData{
			Action:       action,
			Step:         1,
			SessionIndex: i, // Use index to keep callback data small
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildExtendDurationButtons creates buttons for selecting extension duration
func BuildExtendDurationButtons(sessionIndex int) tgbotapi.InlineKeyboardMarkup {
	durations := []int{5, 15, 30, 60, 120}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create two rows
	row1 := []tgbotapi.InlineKeyboardButton{}
	row2 := []tgbotapi.InlineKeyboardButton{}

	for i, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:       "extend",
			Step:         2,
			SessionIndex: sessionIndex, // Use index instead of full UUID
			Duration:     duration,
		})

		label := fmt.Sprintf("+%d", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)

		if i < 3 {
			row1 = append(row1, btn)
		} else {
			row2 = append(row2, btn)
		}
	}

	rows = append(rows, row1, row2)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "extend", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildMainMenuButtons creates main menu shortcut buttons
func BuildMainMenuButtons() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Today", "/today"),
			tgbotapi.NewInlineKeyboardButtonData("➕ New Session", "/newsession"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏱ Extend", "/extend"),
		),
	)
}

// BuildQuickActionsButtons creates compact action buttons for attaching to responses
func BuildQuickActionsButtons() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Today", "/today"),
			tgbotapi.NewInlineKeyboardButtonData("➕ New", "/newsession"),
			tgbotapi.NewInlineKeyboardButtonData("⏱ Extend", "/extend"),
		),
	)
}
