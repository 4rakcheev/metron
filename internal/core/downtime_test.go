package core

import (
	"testing"
	"time"
)

// TestIsInDowntime_SameDay tests downtime within the same day (e.g., 08:00-17:00)
func TestIsInDowntime_SameDay(t *testing.T) {
	schedule := &DowntimeSchedule{
		StartHour:   8,
		StartMinute: 0,
		EndHour:     17,
		EndMinute:   0,
	}

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
			testTime := time.Date(2024, 1, 1, tt.hour, tt.minute, 0, 0, loc)
			got := service.IsInDowntime(testTime)
			if got != tt.wantActive {
				t.Errorf("IsInDowntime(%02d:%02d) = %v, want %v", tt.hour, tt.minute, got, tt.wantActive)
			}
		})
	}
}

// TestIsInDowntime_Overnight tests downtime crossing midnight (e.g., 22:00-10:00)
func TestIsInDowntime_Overnight(t *testing.T) {
	schedule := &DowntimeSchedule{
		StartHour:   22,
		StartMinute: 0,
		EndHour:     10,
		EndMinute:   0,
	}

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
			testTime := time.Date(2024, 1, 1, tt.hour, tt.minute, 0, 0, loc)
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
	schedule := &DowntimeSchedule{
		StartHour:   22,
		StartMinute: 0,
		EndHour:     10,
		EndMinute:   0,
	}

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
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc)
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
	schedule := &DowntimeSchedule{
		StartHour:   22,
		StartMinute: 0,
		EndHour:     10,
		EndMinute:   0,
	}

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
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc)
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
	schedule := &DowntimeSchedule{
		StartHour:   22,
		StartMinute: 0,
		EndHour:     10,
		EndMinute:   0,
	}

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
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, loc)
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

	t.Run("enabled with schedule", func(t *testing.T) {
		schedule := &DowntimeSchedule{
			StartHour: 22,
			EndHour:   10,
		}
		service := NewDowntimeService(schedule, loc)
		if !service.IsEnabled() {
			t.Error("IsEnabled() = false, want true when schedule provided")
		}
	})

	t.Run("disabled without schedule", func(t *testing.T) {
		service := NewDowntimeService(nil, loc)
		if service.IsEnabled() {
			t.Error("IsEnabled() = true, want false when schedule is nil")
		}
	})
}
