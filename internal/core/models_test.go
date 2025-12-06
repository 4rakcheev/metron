package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChild_Validate(t *testing.T) {
	tests := []struct {
		name    string
		child   Child
		wantErr error
	}{
		{
			name: "valid child",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 60,
				WeekendLimit: 120,
			},
			wantErr: nil,
		},
		{
			name: "valid child with break rule",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 60,
				WeekendLimit: 120,
				BreakRule: &BreakRule{
					BreakAfterMinutes:   30,
					BreakDurationMinutes: 10,
				},
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			child: Child{
				ID:           "child1",
				WeekdayLimit: 60,
				WeekendLimit: 120,
			},
			wantErr: ErrInvalidName,
		},
		{
			name: "zero weekday limit",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 0,
				WeekendLimit: 120,
			},
			wantErr: ErrInvalidWeekdayLimit,
		},
		{
			name: "negative weekend limit",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 60,
				WeekendLimit: -10,
			},
			wantErr: ErrInvalidWeekendLimit,
		},
		{
			name: "invalid break rule - zero break after",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 60,
				WeekendLimit: 120,
				BreakRule: &BreakRule{
					BreakAfterMinutes:   0,
					BreakDurationMinutes: 10,
				},
			},
			wantErr: ErrInvalidBreakRule,
		},
		{
			name: "invalid break rule - zero break duration",
			child: Child{
				ID:           "child1",
				Name:         "Alice",
				WeekdayLimit: 60,
				WeekendLimit: 120,
				BreakRule: &BreakRule{
					BreakAfterMinutes:   30,
					BreakDurationMinutes: 0,
				},
			},
			wantErr: ErrInvalidBreakRule,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.child.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChild_GetDailyLimit(t *testing.T) {
	child := Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}

	tests := []struct {
		name      string
		date      time.Time
		wantLimit int
	}{
		{
			name:      "Monday",
			date:      time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), // Monday
			wantLimit: 60,
		},
		{
			name:      "Saturday",
			date:      time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC), // Saturday
			wantLimit: 120,
		},
		{
			name:      "Sunday",
			date:      time.Date(2025, 12, 7, 0, 0, 0, 0, time.UTC), // Sunday
			wantLimit: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := child.GetDailyLimit(tt.date)
			assert.Equal(t, tt.wantLimit, limit)
		})
	}
}

func TestSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		session Session
		wantErr error
	}{
		{
			name: "valid session",
			session: Session{
				ID:               "session1",
				DeviceType:       "tv",
				DeviceID:         "tv1",
				ChildIDs:         []string{"child1"},
				ExpectedDuration: 30,
			},
			wantErr: nil,
		},
		{
			name: "valid session with multiple children",
			session: Session{
				ID:               "session1",
				DeviceType:       "tv",
				DeviceID:         "tv1",
				ChildIDs:         []string{"child1", "child2"},
				ExpectedDuration: 30,
			},
			wantErr: nil,
		},
		{
			name: "empty device type",
			session: Session{
				ID:               "session1",
				DeviceID:         "tv1",
				ChildIDs:         []string{"child1"},
				ExpectedDuration: 30,
			},
			wantErr: ErrInvalidDeviceType,
		},
		{
			name: "no children",
			session: Session{
				ID:               "session1",
				DeviceType:       "tv",
				DeviceID:         "tv1",
				ChildIDs:         []string{},
				ExpectedDuration: 30,
			},
			wantErr: ErrNoChildren,
		},
		{
			name: "zero duration",
			session: Session{
				ID:               "session1",
				DeviceType:       "tv",
				DeviceID:         "tv1",
				ChildIDs:         []string{"child1"},
				ExpectedDuration: 0,
			},
			wantErr: ErrInvalidDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSession_IsActive(t *testing.T) {
	tests := []struct {
		name       string
		status     SessionStatus
		wantActive bool
	}{
		{
			name:       "active session",
			status:     SessionStatusActive,
			wantActive: true,
		},
		{
			name:       "paused session",
			status:     SessionStatusPaused,
			wantActive: false,
		},
		{
			name:       "completed session",
			status:     SessionStatusCompleted,
			wantActive: false,
		},
		{
			name:       "expired session",
			status:     SessionStatusExpired,
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Session{Status: tt.status}
			assert.Equal(t, tt.wantActive, session.IsActive())
		})
	}
}

func TestSession_IsInBreak(t *testing.T) {
	now := time.Now()
	future := now.Add(10 * time.Minute)
	past := now.Add(-10 * time.Minute)

	tests := []struct {
		name        string
		breakEndsAt *time.Time
		wantInBreak bool
	}{
		{
			name:        "no break",
			breakEndsAt: nil,
			wantInBreak: false,
		},
		{
			name:        "break in future",
			breakEndsAt: &future,
			wantInBreak: true,
		},
		{
			name:        "break in past",
			breakEndsAt: &past,
			wantInBreak: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Session{BreakEndsAt: tt.breakEndsAt}
			assert.Equal(t, tt.wantInBreak, session.IsInBreak())
		})
	}
}

func TestSession_NeedsBreak(t *testing.T) {
	now := time.Now()
	breakRule := &BreakRule{
		BreakAfterMinutes:   30,
		BreakDurationMinutes: 10,
	}

	tests := []struct {
		name        string
		session     Session
		breakRule   *BreakRule
		wantBreak   bool
	}{
		{
			name: "no break rule",
			session: Session{
				StartTime: now.Add(-40 * time.Minute),
			},
			breakRule: nil,
			wantBreak: false,
		},
		{
			name: "needs break - since start",
			session: Session{
				StartTime: now.Add(-31 * time.Minute),
			},
			breakRule: breakRule,
			wantBreak: true,
		},
		{
			name: "no break needed - since start",
			session: Session{
				StartTime: now.Add(-20 * time.Minute),
			},
			breakRule: breakRule,
			wantBreak: false,
		},
		{
			name: "needs break - since last break",
			session: Session{
				StartTime:   now.Add(-60 * time.Minute),
				LastBreakAt: timePtr(now.Add(-31 * time.Minute)),
			},
			breakRule: breakRule,
			wantBreak: true,
		},
		{
			name: "no break needed - since last break",
			session: Session{
				StartTime:   now.Add(-60 * time.Minute),
				LastBreakAt: timePtr(now.Add(-20 * time.Minute)),
			},
			breakRule: breakRule,
			wantBreak: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needsBreak := tt.session.NeedsBreak(tt.breakRule)
			assert.Equal(t, tt.wantBreak, needsBreak)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
