// Time formatting utilities

/**
 * Formats minutes into hours and minutes display
 * @param minutes - Total minutes
 * @returns Formatted string like "1h 15m" or "45m"
 */
export function formatMinutes(minutes: number): string {
  if (minutes < 0) return '0m';

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  if (hours === 0) {
    return `${mins}m`;
  }

  if (mins === 0) {
    return `${hours}h`;
  }

  return `${hours}h ${mins}m`;
}

/**
 * Formats minutes into a more detailed display for large time values
 * @param minutes - Total minutes
 * @returns Formatted string with separate hour and minute components
 */
export function formatMinutesDetailed(minutes: number): {
  hours: number;
  minutes: number;
  formatted: string;
} {
  if (minutes < 0) minutes = 0;

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  return {
    hours,
    minutes: mins,
    formatted: formatMinutes(minutes),
  };
}

/**
 * Formats an RFC3339 timestamp as a local clock time ("HH:MM").
 * Adds a short date when the time is not today.
 * @param iso - RFC3339 timestamp
 * @returns Formatted string like "18:30" or "Tue 18:30"
 */
export function formatClockTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';

  const time = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (date.toDateString() === new Date().toDateString()) {
    return time;
  }

  const day = date.toLocaleDateString([], { weekday: 'short', day: 'numeric', month: 'short' });
  return `${day} ${time}`;
}
