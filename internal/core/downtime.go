package core

import (
	"time"
)

// DowntimeSchedule defines the time period when downtime is active
type DowntimeSchedule struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
}

// DowntimeService manages downtime schedule logic
type DowntimeService struct {
	schedule *DowntimeSchedule
	timezone *time.Location
}

// NewDowntimeService creates a new downtime service
// If schedule is nil, the service is disabled and all checks will pass
func NewDowntimeService(schedule *DowntimeSchedule, timezone *time.Location) *DowntimeService {
	return &DowntimeService{
		schedule: schedule,
		timezone: timezone,
	}
}

// IsEnabled returns true if downtime schedule is configured
func (d *DowntimeService) IsEnabled() bool {
	return d.schedule != nil
}

// IsInDowntime checks if the given time falls within the downtime period
func (d *DowntimeService) IsInDowntime(t time.Time) bool {
	if !d.IsEnabled() {
		return false
	}

	// Convert to configured timezone
	localTime := t.In(d.timezone)

	// Calculate minutes since midnight
	currentMinutes := localTime.Hour()*60 + localTime.Minute()
	startMinutes := d.schedule.StartHour*60 + d.schedule.StartMinute
	endMinutes := d.schedule.EndHour*60 + d.schedule.EndMinute

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

	// Calculate the end time for today
	endTime := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		d.schedule.EndHour,
		d.schedule.EndMinute,
		0, 0,
		d.timezone,
	)

	startMinutes := d.schedule.StartHour*60 + d.schedule.StartMinute
	endMinutes := d.schedule.EndHour*60 + d.schedule.EndMinute

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

	// Calculate the start time for today
	startTime := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		d.schedule.StartHour,
		d.schedule.StartMinute,
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
