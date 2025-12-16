package bot

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CallbackData represents the data embedded in callback buttons
type CallbackData struct {
	Action       string `json:"a"`             // Action type (newsession, manage, etc)
	SubAction    string `json:"sa,omitempty"`  // Sub-action (extend, stop, add_kid)
	Step         int    `json:"s,omitempty"`   // Current step in flow
	ChildID      string `json:"c,omitempty"`   // Child ID (resolved from index)
	ChildIndex   int    `json:"ci,omitempty"`  // Child index in list (for compact callback)
	Device       string `json:"d,omitempty"`   // Device ID
	Duration     int    `json:"m,omitempty"`   // Duration in minutes
	Session      string `json:"ses,omitempty"` // Session ID (resolved from index)
	SessionIndex int    `json:"si,omitempty"`  // Session index in list (for compact callback)
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
func BuildDevicesButtons(devices []Device, action string, step int, childIndex int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, device := range devices {
		emoji := getDeviceEmoji(device.Type)
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: childIndex, // Use index to keep callback data small
			Device:     device.ID,  // Use device ID, not type
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
		MarshalCallback(CallbackData{Action: action, Step: step - 1, ChildIndex: childIndex}),
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
func BuildDurationButtons(action string, step int, childIndex int, device string) tgbotapi.InlineKeyboardMarkup {
	durations := []int{5, 15, 30, 60, 120}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create two rows: [5, 15, 30] and [60, 120]
	row1 := []tgbotapi.InlineKeyboardButton{}
	row2 := []tgbotapi.InlineKeyboardButton{}

	for i, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: childIndex, // Use index to keep callback data small
			Device:     device,
			Duration:   duration,
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
		MarshalCallback(CallbackData{Action: action, Step: step - 1, ChildIndex: childIndex, Device: device}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildSessionsButtons creates buttons for selecting an active session (legacy)
// Deprecated: Use BuildSessionManagementButtons for better UX
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

// BuildSessionManagementButtons creates buttons for managing active sessions
// Each session gets 3 action buttons: Extend, Stop, Add Kid
func BuildSessionManagementButtons(sessions []Session, childrenMap map[string]Child) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, session := range sessions {
		deviceEmoji := getDeviceEmoji(session.DeviceType)
		displayName := getDeviceDisplayName(session.DeviceType)

		// Get child names for this session
		var childNames []string
		for _, childID := range session.ChildIDs {
			if child, ok := childrenMap[childID]; ok {
				childEmoji := getChildEmoji(child.Name)
				childNames = append(childNames, childEmoji+" "+child.Name)
			}
		}

		// Header row with session info (not clickable)
		_, remaining := calculateSessionEnd(session)
		sessionLabel := fmt.Sprintf("%d. %s %s", i+1, deviceEmoji, displayName)
		if len(childNames) > 0 {
			sessionLabel += fmt.Sprintf(" · %d min", remaining)
		}

		// Action buttons row: [Extend] [Stop] [Add Kid]
		extendBtn := tgbotapi.NewInlineKeyboardButtonData(
			"⏱ Extend",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "extend",
				Step:         1,
				SessionIndex: i,
			}),
		)

		stopBtn := tgbotapi.NewInlineKeyboardButtonData(
			"🛑 Stop",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "stop",
				Step:         1,
				SessionIndex: i,
			}),
		)

		addKidBtn := tgbotapi.NewInlineKeyboardButtonData(
			"👶 Add Kid",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "add_kid",
				Step:         1,
				SessionIndex: i,
			}),
		)

		// Add session label as a single-button row (for visual grouping)
		labelBtn := tgbotapi.NewInlineKeyboardButtonData(
			sessionLabel,
			MarshalCallback(CallbackData{Action: "noop"}), // No-op callback
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{labelBtn})

		// Add action buttons
		rows = append(rows, []tgbotapi.InlineKeyboardButton{extendBtn, stopBtn, addKidBtn})
	}

	// Grant Reward button
	rewardBtn := tgbotapi.NewInlineKeyboardButtonData(
		"🎁 Grant Reward",
		MarshalCallback(CallbackData{Action: "reward", Step: 0}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{rewardBtn})

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildAddKidButtons creates buttons for selecting which child to add to a session
func BuildAddKidButtons(sessionIndex int, availableChildren []Child, alreadyShared bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Show available children to add
	for i, child := range availableChildren {
		emoji := getChildEmoji(child.Name)
		callback := MarshalCallback(CallbackData{
			Action:       "manage",
			SubAction:    "add_kid",
			Step:         2,
			SessionIndex: sessionIndex,
			ChildIndex:   i,
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, child.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// "Mark as Shared" button if not already shared
	if !alreadyShared && len(availableChildren) > 0 {
		callback := MarshalCallback(CallbackData{
			Action:       "manage",
			SubAction:    "add_kid",
			Step:         2,
			SessionIndex: sessionIndex,
			ChildIndex:   -1, // Special marker for "all available children"
		})

		btn := tgbotapi.NewInlineKeyboardButtonData("👨‍👩‍👧 Mark as Shared (All)", callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "manage", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

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
			Action:       "manage",
			SubAction:    "extend",
			Step:         2,
			SessionIndex: sessionIndex,
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
		MarshalCallback(CallbackData{Action: "manage", Step: 0}),
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

// BuildRewardDurationButtons creates buttons for selecting reward duration
func BuildRewardDurationButtons(childIndex int) tgbotapi.InlineKeyboardMarkup {
	durations := []int{15, 30, 60}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create one row with all three reward options
	row := []tgbotapi.InlineKeyboardButton{}

	for _, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:     "reward",
			Step:       2,
			ChildIndex: childIndex,
			Duration:   duration,
		})

		label := fmt.Sprintf("+%d min", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		row = append(row, btn)
	}

	rows = append(rows, row)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "reward", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
