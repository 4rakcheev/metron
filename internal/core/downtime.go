package core

import (
	"context"
	"time"
)

// DaySchedule defines start/end times for a specific day type
type DaySchedule struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
}

// DowntimeSchedule defines the time periods when downtime is active
// Supports separate schedules for weekdays (Mon-Fri) and weekends (Sat-Sun)
type DowntimeSchedule struct {
	Weekday *DaySchedule // Mon-Fri schedule
	Weekend *DaySchedule // Sat-Sun schedule
}

// DowntimeSkipStorage defines the interface for skip date persistence
type DowntimeSkipStorage interface {
	GetDowntimeSkipDate(ctx context.Context) (*time.Time, error)
	SetDowntimeSkipDate(ctx context.Context, date time.Time) error
}

// DowntimeService manages downtime schedule logic
type DowntimeService struct {
	schedule    *DowntimeSchedule
	timezone    *time.Location
	skipStorage DowntimeSkipStorage
}

// NewDowntimeService creates a new downtime service
// If schedule is nil, the service is disabled and all checks will pass
func NewDowntimeService(schedule *DowntimeSchedule, timezone *time.Location) *DowntimeService {
	return &DowntimeService{
		schedule: schedule,
		timezone: timezone,
	}
}

// SetSkipStorage sets the storage for downtime skip feature
func (d *DowntimeService) SetSkipStorage(storage DowntimeSkipStorage) {
	d.skipStorage = storage
}

// getScheduleForDay returns the appropriate schedule for the given day
func (d *DowntimeService) getScheduleForDay(t time.Time) *DaySchedule {
	if d.schedule == nil {
		return nil
	}

	weekday := t.In(d.timezone).Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return d.schedule.Weekend
	}
	return d.schedule.Weekday
}

// IsEnabled returns true if downtime schedule is configured
func (d *DowntimeService) IsEnabled() bool {
	return d.schedule != nil && (d.schedule.Weekday != nil || d.schedule.Weekend != nil)
}

// IsDowntimeSkippedToday checks if downtime has been skipped for today
func (d *DowntimeService) IsDowntimeSkippedToday(ctx context.Context, now time.Time) bool {
	if d.skipStorage == nil {
		return false
	}

	skipDate, err := d.skipStorage.GetDowntimeSkipDate(ctx)
	if err != nil || skipDate == nil {
		return false
	}

	// Compare dates in configured timezone
	localNow := now.In(d.timezone)
	localSkip := skipDate.In(d.timezone)

	return localNow.Year() == localSkip.Year() &&
		localNow.Month() == localSkip.Month() &&
		localNow.Day() == localSkip.Day()
}

// IsInDowntime checks if the given time falls within the downtime period
func (d *DowntimeService) IsInDowntime(t time.Time) bool {
	return d.IsInDowntimeWithContext(context.Background(), t)
}

// IsInDowntimeWithContext checks if the given time falls within the downtime period
// with support for checking skip status
func (d *DowntimeService) IsInDowntimeWithContext(ctx context.Context, t time.Time) bool {
	if !d.IsEnabled() {
		return false
	}

	// Check if downtime is skipped today
	if d.IsDowntimeSkippedToday(ctx, t) {
		return false
	}

	// Get the schedule for this day
	schedule := d.getScheduleForDay(t)
	if schedule == nil {
		return false
	}

	// Convert to configured timezone
	localTime := t.In(d.timezone)

	// Calculate minutes since midnight
	currentMinutes := localTime.Hour()*60 + localTime.Minute()
	startMinutes := schedule.StartHour*60 + schedule.StartMinute
	endMinutes := schedule.EndHour*60 + schedule.EndMinute

	if startMinutes > endMinutes {
		// Overnight period (e.g., 22:00 to 10:00)
		// In downtime if >= start OR < end
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	// Same-day period (e.g., 08:00 to 17:00)
	// In downtime if >= start AND < end
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// IsChildInDowntime checks if downtime is active for a specific child
// Returns true only if:
// 1. Downtime schedule is configured
// 2. Current time is in downtime period
// 3. Child has downtime enabled
func (d *DowntimeService) IsChildInDowntime(child *Child, now time.Time) bool {
	if !d.IsEnabled() {
		return false
	}

	if !child.DowntimeEnabled {
		return false
	}

	return d.IsInDowntime(now)
}

// GetCurrentDowntimeEnd returns when the current downtime period ends
// Returns zero time if not currently in downtime or downtime is disabled
func (d *DowntimeService) GetCurrentDowntimeEnd(now time.Time) time.Time {
	if !d.IsEnabled() || !d.IsInDowntime(now) {
		return time.Time{}
	}

	localNow := now.In(d.timezone)
	schedule := d.getScheduleForDay(now)
	if schedule == nil {
		return time.Time{}
	}

	// Calculate the end time for today
	endTime := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		schedule.EndHour,
		schedule.EndMinute,
		0, 0,
		d.timezone,
	)

	startMinutes := schedule.StartHour*60 + schedule.StartMinute
	endMinutes := schedule.EndHour*60 + schedule.EndMinute

	if startMinutes > endMinutes {
		// Overnight period
		currentMinutes := localNow.Hour()*60 + localNow.Minute()

		// If we're before the end time (morning), return today's end time
		if currentMinutes < endMinutes {
			return endTime
		}

		// If we're after the start time (evening), return tomorrow's end time
		return endTime.Add(24 * time.Hour)
	}

	// Same-day period
	return endTime
}

// GetNextDowntimeStart returns when the next downtime period starts
// Returns zero time if downtime is disabled
func (d *DowntimeService) GetNextDowntimeStart(now time.Time) time.Time {
	if !d.IsEnabled() {
		return time.Time{}
	}

	localNow := now.In(d.timezone)
	schedule := d.getScheduleForDay(now)
	if schedule == nil {
		return time.Time{}
	}

	// Calculate the start time for today
	startTime := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		schedule.StartHour,
		schedule.StartMinute,
		0, 0,
		d.timezone,
	)

	// If the start time hasn't passed yet today, return it
	if localNow.Before(startTime) {
		return startTime
	}

	// Otherwise, return tomorrow's start time
	return startTime.Add(24 * time.Hour)
}
