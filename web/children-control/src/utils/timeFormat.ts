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
