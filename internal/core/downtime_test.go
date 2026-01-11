package core

import (
	"testing"
	"time"
)

// Helper to create a schedule with same times for both weekday and weekend
func newUnifiedSchedule(startHour, startMinute, endHour, endMinute int) *DowntimeSchedule {
	daySched := &DaySchedule{
		StartHour:   startHour,
		StartMinute: startMinute,
		EndHour:     endHour,
		EndMinute:   endMinute,
	}
	return &DowntimeSchedule{
		Weekday: daySched,
		Weekend: daySched,
	}
}

// TestIsInDowntime_SameDay tests downtime within the same day (e.g., 08:00-17:00)
func TestIsInDowntime_SameDay(t *testing.T) {
	schedule := newUnifiedSchedule(8, 0, 17, 0)

	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(schedule, loc)

	tests := []struct {
		hour       int
		minute     int
		wantActive bool
		desc       string
	}{
		{7, 59, false, "before downtime starts"},
		{8, 0, true, "exactly at start"},
		{12, 30, true, "middle of downtime"},
		{16, 59, true, "just before end"},
		{17, 0, false, "exactly at end"},
		{18, 0, false, "after downtime ends"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Use a Monday (weekday) for consistent testing
			testTime := time.Date(2024, 1, 1, tt.hour, tt.minute, 0, 0, loc) // Monday
			got := service.IsInDowntime(testTime)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%02d:%02d) = %v, want %v", tt.hour, tt.minute, got, tt.wantActive)
			}
		})
	}
}

// TestIsInDowntime_Overnight tests downtime crossing midnight (e.g., 22:00-10:00)
func TestIsInDowntime_Overnight(t *testing.T) {
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(schedule, loc)

	tests := []struct {
		hour       int
		minute     int
		wantActive bool
		desc       string
	}{
		{21, 59, false, "before downtime starts (evening)"},
		{22, 0, true, "exactly at start (evening)"},
		{23, 30, true, "late evening"},
		{0, 0, true, "midnight"},
		{5, 30, true, "early morning"},
		{9, 59, true, "just before end (morning)"},
		{10, 0, false, "exactly at end (morning)"},
		{15, 0, false, "afternoon (not in downtime)"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Use a Monday (weekday) for consistent testing
			testTime := time.Date(2024, 1, 1, tt.hour, tt.minute, 0, 0, loc) // Monday
			got := service.IsInDowntime(testTime)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%02d:%02d) = %v, want %v", tt.hour, tt.minute, got, tt.wantActive)
			}
		})
	}
}

// TestIsInDowntime_Disabled tests that disabled service always returns false
func TestIsInDowntime_Disabled(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(nil, loc) // nil schedule = disabled

	tests := []struct {
		hour int
		desc string
	}{
		{0, "midnight"},
		{8, "morning"},
		{12, "noon"},
		{18, "evening"},
		{22, "night"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc)
			got := service.IsInDowntime(testTime)
			if got != false {
				t.Errorf("IsInDowntime(%02d:00) = %v, want false (service disabled)", tt.hour, got)
			}
		})
	}
}

// TestIsChildInDowntime tests per-child downtime checks
func TestIsChildInDowntime(t *testing.T) {
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(schedule, loc)

	tests := []struct {
		hour            int
		downtimeEnabled bool
		wantActive      bool
		desc            string
	}{
		{23, true, true, "enabled child during downtime"},
		{23, false, false, "disabled child during downtime"},
		{15, true, false, "enabled child outside downtime"},
		{15, false, false, "disabled child outside downtime"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			child := &Child{
				ID:              "test-child",
				Name:            "Test",
				DowntimeEnabled: tt.downtimeEnabled,
			}
			// Use a Monday (weekday) for consistent testing
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc) // Monday
			got := service.IsChildInDowntime(child, testTime)
			if got != tt.wantActive {
				t.Errorf("IsChildInDowntime(enabled=%v, time=%02d:00) = %v, want %v",
					tt.downtimeEnabled, tt.hour, got, tt.wantActive)
			}
		})
	}
}

// TestGetCurrentDowntimeEnd tests calculating when downtime ends
func TestGetCurrentDowntimeEnd(t *testing.T) {
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(schedule, loc)

	tests := []struct {
		hour           int
		wantEndHour    int
		wantEndMinute  int
		desc           string
		expectZeroTime bool
	}{
		{23, 10, 0, "evening - ends tomorrow morning", false},
		{5, 10, 0, "early morning - ends today", false},
		{15, 0, 0, "afternoon - not in downtime", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Use a Monday (weekday) for consistent testing
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc) // Monday
			got := service.GetCurrentDowntimeEnd(testTime)

			if tt.expectZeroTime {
				if !got.IsZero() {
					t.Errorf("GetCurrentDowntimeEnd(%02d:00) should return zero time, got %v",
						tt.hour, got)
				}
				return
			}

			if got.Hour() != tt.wantEndHour || got.Minute() != tt.wantEndMinute {
				t.Errorf("GetCurrentDowntimeEnd(%02d:00) = %02d:%02d, want %02d:%02d",
					tt.hour, got.Hour(), got.Minute(), tt.wantEndHour, tt.wantEndMinute)
			}
		})
	}
}

// TestGetNextDowntimeStart tests calculating when next downtime starts
func TestGetNextDowntimeStart(t *testing.T) {
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	loc, _ := time.LoadLocation("UTC")
	service := NewDowntimeService(schedule, loc)

	tests := []struct {
		hour            int
		wantStartHour   int
		wantStartMinute int
		desc            string
		expectToday     bool
	}{
		{15, 22, 0, "afternoon - starts today", true},
		{23, 22, 0, "during downtime - starts tomorrow", false},
		{5, 22, 0, "early morning - starts today", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Use a Monday (weekday) for consistent testing
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc) // Monday
			got := service.GetNextDowntimeStart(testTime)

			if got.Hour() != tt.wantStartHour || got.Minute() != tt.wantStartMinute {
				t.Errorf("GetNextDowntimeStart(%02d:00) = %02d:%02d, want %02d:%02d",
					tt.hour, got.Hour(), got.Minute(), tt.wantStartHour, tt.wantStartMinute)
			}

			if tt.expectToday && got.Day() != testTime.Day() {
				t.Errorf("GetNextDowntimeStart(%02d:00) should be today, got tomorrow", tt.hour)
			}
			if !tt.expectToday && got.Day() == testTime.Day() {
				t.Errorf("GetNextDowntimeStart(%02d:00) should be tomorrow, got today", tt.hour)
			}
		})
	}
}

// TestIsEnabled tests the IsEnabled method
func TestIsEnabled(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	t.Run("enabled with weekday schedule", func(t *testing.T) {
		schedule := &DowntimeSchedule{
			Weekday: &DaySchedule{StartHour: 22, EndHour: 10},
		}
		service := NewDowntimeService(schedule, loc)
		if !service.IsEnabled() {
			t.Error("IsEnabled() = false, want true when weekday schedule provided")
		}
	})

	t.Run("enabled with weekend schedule", func(t *testing.T) {
		schedule := &DowntimeSchedule{
			Weekend: &DaySchedule{StartHour: 23, EndHour: 11},
		}
		service := NewDowntimeService(schedule, loc)
		if !service.IsEnabled() {
			t.Error("IsEnabled() = false, want true when weekend schedule provided")
		}
	})

	t.Run("enabled with both schedules", func(t *testing.T) {
		schedule := &DowntimeSchedule{
			Weekday: &DaySchedule{StartHour: 21, EndHour: 10},
			Weekend: &DaySchedule{StartHour: 22, EndHour: 10},
		}
		service := NewDowntimeService(schedule, loc)
		if !service.IsEnabled() {
			t.Error("IsEnabled() = false, want true when both schedules provided")
		}
	})

	t.Run("disabled without schedule", func(t *testing.T) {
		service := NewDowntimeService(nil, loc)
		if service.IsEnabled() {
			t.Error("IsEnabled() = true, want false when schedule is nil")
		}
	})

	t.Run("disabled with empty schedule", func(t *testing.T) {
		schedule := &DowntimeSchedule{}
		service := NewDowntimeService(schedule, loc)
		if service.IsEnabled() {
			t.Error("IsEnabled() = true, want false when schedule has no day schedules")
		}
	})
}

// TestIsInDowntime_RigaTimezone tests downtime with Europe/Riga timezone
// This specifically tests the user's reported issue: at 21:01 Riga time,
// downtime starting at 22:00 should NOT be active
func TestIsInDowntime_RigaTimezone(t *testing.T) {
	// Downtime schedule: 22:00 - 10:00 (overnight)
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	riga, err := time.LoadLocation("Europe/Riga")
	if err != nil {
		t.Fatalf("Failed to load Europe/Riga timezone: %v", err)
	}
	service := NewDowntimeService(schedule, riga)

	tests := []struct {
		hour       int
		minute     int
		wantActive bool
		desc       string
	}{
		{21, 1, false, "21:01 Riga - before downtime starts"},
		{21, 59, false, "21:59 Riga - just before downtime"},
		{22, 0, true, "22:00 Riga - exactly at start"},
		{22, 1, true, "22:01 Riga - just after start"},
		{23, 0, true, "23:00 Riga - evening downtime"},
		{9, 59, true, "09:59 Riga - just before end"},
		{10, 0, false, "10:00 Riga - exactly at end"},
		{15, 0, false, "15:00 Riga - afternoon, not in downtime"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create time in Riga timezone (use a Thursday in January - weekday)
			testTime := time.Date(2024, 1, 4, tt.hour, tt.minute, 0, 0, riga)
			got := service.IsInDowntime(testTime)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%02d:%02d Riga) = %v, want %v",
					tt.hour, tt.minute, got, tt.wantActive)
			}
		})
	}
}

// TestIsInDowntime_UTCServerWithRigaTimezone tests the scenario where
// server runs in UTC but downtime is configured for Riga timezone
func TestIsInDowntime_UTCServerWithRigaTimezone(t *testing.T) {
	// Downtime schedule: 22:00 - 10:00 Riga time
	schedule := newUnifiedSchedule(22, 0, 10, 0)

	riga, err := time.LoadLocation("Europe/Riga")
	if err != nil {
		t.Fatalf("Failed to load Europe/Riga timezone: %v", err)
	}
	service := NewDowntimeService(schedule, riga)

	// In January, Europe/Riga is UTC+2 (EET)
	// So 21:01 Riga = 19:01 UTC
	// And 22:00 Riga = 20:00 UTC

	tests := []struct {
		utcHour    int
		utcMinute  int
		wantActive bool
		desc       string
	}{
		// 19:01 UTC = 21:01 Riga (before 22:00 downtime)
		{19, 1, false, "19:01 UTC (21:01 Riga) - before downtime"},
		// 19:59 UTC = 21:59 Riga (still before downtime)
		{19, 59, false, "19:59 UTC (21:59 Riga) - just before downtime"},
		// 20:00 UTC = 22:00 Riga (exactly at downtime start)
		{20, 0, true, "20:00 UTC (22:00 Riga) - exactly at start"},
		// 20:01 UTC = 22:01 Riga (in downtime)
		{20, 1, true, "20:01 UTC (22:01 Riga) - just after start"},
		// 08:00 UTC = 10:00 Riga (exactly at end)
		{8, 0, false, "08:00 UTC (10:00 Riga) - exactly at end"},
		// 07:59 UTC = 09:59 Riga (just before end)
		{7, 59, true, "07:59 UTC (09:59 Riga) - just before end"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create time in UTC (simulating server time)
			// Use January 4, 2024 (Thursday - weekday)
			testTime := time.Date(2024, 1, 4, tt.utcHour, tt.utcMinute, 0, 0, time.UTC)
			got := service.IsInDowntime(testTime)
			if got != tt.wantActive {
				localTime := testTime.In(riga)
				t.Errorf("IsInDowntime(%02d:%02d UTC = %02d:%02d Riga) = %v, want %v",
					tt.utcHour, tt.utcMinute,
					localTime.Hour(), localTime.Minute(),
					got, tt.wantActive)
			}
		})
	}
}

// TestWeekdayWeekendSchedules tests legacy weekday/weekend fallback schedules
// With fallback: weekday = Mon-Fri, weekend = Sat-Sun
func TestWeekdayWeekendSchedules(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	// Weekday: 21:00-10:00 (Mon-Fri)
	// Weekend: 22:00-11:00 (Sat-Sun)
	schedule := &DowntimeSchedule{
		Weekday: &DaySchedule{StartHour: 21, StartMinute: 0, EndHour: 10, EndMinute: 0},
		Weekend: &DaySchedule{StartHour: 22, StartMinute: 0, EndHour: 11, EndMinute: 0},
	}
	service := NewDowntimeService(schedule, loc)

	// 2024-01-01 is Monday, 2024-01-05 is Friday, 2024-01-06 is Saturday, 2024-01-07 is Sunday
	tests := []struct {
		date       time.Time
		wantActive bool
		desc       string
	}{
		// Monday - weekday schedule 21:00
		{time.Date(2024, 1, 1, 21, 0, 0, 0, loc), true, "Monday 21:00 - weekday schedule active"},
		{time.Date(2024, 1, 1, 20, 59, 0, 0, loc), false, "Monday 20:59 - before weekday downtime"},

		// Thursday - weekday schedule 21:00
		{time.Date(2024, 1, 4, 21, 0, 0, 0, loc), true, "Thursday 21:00 - weekday schedule active"},

		// Friday - weekday schedule 21:00 (with fallback, Fri is weekday)
		{time.Date(2024, 1, 5, 21, 0, 0, 0, loc), true, "Friday 21:00 - weekday schedule active"},
		{time.Date(2024, 1, 5, 21, 30, 0, 0, loc), true, "Friday 21:30 - weekday schedule active"},

		// Saturday - weekend schedule 22:00
		{time.Date(2024, 1, 6, 21, 30, 0, 0, loc), false, "Saturday 21:30 - before weekend downtime"},
		{time.Date(2024, 1, 6, 22, 0, 0, 0, loc), true, "Saturday 22:00 - weekend schedule active"},
		{time.Date(2024, 1, 6, 10, 30, 0, 0, loc), true, "Saturday 10:30 - still in weekend downtime (ends 11:00)"},
		{time.Date(2024, 1, 6, 11, 0, 0, 0, loc), false, "Saturday 11:00 - weekend downtime ended"},

		// Sunday - weekend schedule 22:00
		{time.Date(2024, 1, 7, 21, 30, 0, 0, loc), false, "Sunday 21:30 - before weekend downtime"},
		{time.Date(2024, 1, 7, 22, 0, 0, 0, loc), true, "Sunday 22:00 - weekend schedule active"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := service.IsInDowntime(tt.date)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%v) = %v, want %v", tt.date.Format("Mon 15:04"), got, tt.wantActive)
			}
		})
	}
}

// TestPerDaySchedules tests explicit per-day schedule configuration
func TestPerDaySchedules(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	// School night schedule: Sun-Thu at 21:00 (before school days)
	// Non-school night schedule: Fri-Sat at 22:00 (before non-school days)
	schoolNight := &DaySchedule{StartHour: 21, StartMinute: 0, EndHour: 10, EndMinute: 0}
	nonSchoolNight := &DaySchedule{StartHour: 22, StartMinute: 0, EndHour: 11, EndMinute: 0}

	schedule := &DowntimeSchedule{
		Sunday:    schoolNight,    // Before Monday (school)
		Monday:    schoolNight,    // Before Tuesday (school)
		Tuesday:   schoolNight,    // Before Wednesday (school)
		Wednesday: schoolNight,    // Before Thursday (school)
		Thursday:  schoolNight,    // Before Friday (school)
		Friday:    nonSchoolNight, // Before Saturday (no school)
		Saturday:  nonSchoolNight, // Before Sunday (no school)
	}
	service := NewDowntimeService(schedule, loc)

	// 2024-01-01 is Monday, 2024-01-05 is Friday, 2024-01-06 is Saturday, 2024-01-07 is Sunday
	tests := []struct {
		date       time.Time
		wantActive bool
		desc       string
	}{
		// Monday (school night) - 21:00
		{time.Date(2024, 1, 1, 21, 0, 0, 0, loc), true, "Monday 21:00 - school night active"},
		{time.Date(2024, 1, 1, 20, 59, 0, 0, loc), false, "Monday 20:59 - before school night"},

		// Thursday (school night) - 21:00
		{time.Date(2024, 1, 4, 21, 0, 0, 0, loc), true, "Thursday 21:00 - school night active"},

		// Friday (non-school night) - 22:00
		{time.Date(2024, 1, 5, 21, 30, 0, 0, loc), false, "Friday 21:30 - before non-school night"},
		{time.Date(2024, 1, 5, 22, 0, 0, 0, loc), true, "Friday 22:00 - non-school night active"},

		// Saturday (non-school night) - 22:00
		{time.Date(2024, 1, 6, 21, 30, 0, 0, loc), false, "Saturday 21:30 - before non-school night"},
		{time.Date(2024, 1, 6, 22, 0, 0, 0, loc), true, "Saturday 22:00 - non-school night active"},

		// Sunday (school night) - 21:00
		{time.Date(2024, 1, 7, 21, 0, 0, 0, loc), true, "Sunday 21:00 - school night active"},
		{time.Date(2024, 1, 7, 21, 30, 0, 0, loc), true, "Sunday 21:30 - school night active"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := service.IsInDowntime(tt.date)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%v) = %v, want %v", tt.date.Format("Mon 15:04"), got, tt.wantActive)
			}
		})
	}
}
