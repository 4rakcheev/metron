// Time formatting utilities

import { getLang, getLocale, translate, type Lang } from '../i18n';

/**
 * Formats minutes into hours and minutes display
 * @param minutes - Total minutes
 * @param lang - Language (defaults to the current app language)
 * @returns Formatted string like "1 ч 15 мин" / "1h 15m" or "45 мин" / "45m"
 */
export function formatMinutes(minutes: number, lang: Lang = getLang()): string {
  if (minutes < 0) minutes = 0;

  const h = Math.floor(minutes / 60);
  const m = minutes % 60;

  if (h === 0) {
    return translate('time.minutes', { m }, lang);
  }

  if (m === 0) {
    return translate('time.hours', { h }, lang);
  }

  return translate('time.hoursMinutes', { h, m }, lang);
}

/**
 * Formats minutes into a more detailed display for large time values
 * @param minutes - Total minutes
 * @param lang - Language (defaults to the current app language)
 * @returns Formatted string with separate hour and minute components
 */
export function formatMinutesDetailed(minutes: number, lang: Lang = getLang()): {
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
    formatted: formatMinutes(minutes, lang),
  };
}

/**
 * Formats an RFC3339 timestamp as a local clock time only ("HH:MM").
 * @param iso - RFC3339 timestamp
 * @param lang - Language (defaults to the current app language)
 */
export function formatTimeOnly(iso: string, lang: Lang = getLang()): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString(getLocale(lang), { hour: '2-digit', minute: '2-digit' });
}

/**
 * Formats an RFC3339 timestamp as a local clock time ("HH:MM").
 * Adds a short date when the time is not today.
 * @param iso - RFC3339 timestamp
 * @param lang - Language (defaults to the current app language)
 * @returns Formatted string like "18:30" or "вт, 17 нояб. 18:30"
 */
export function formatClockTime(iso: string, lang: Lang = getLang()): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';

  const time = formatTimeOnly(iso, lang);
  if (date.toDateString() === new Date().toDateString()) {
    return time;
  }

  const day = date.toLocaleDateString(getLocale(lang), { weekday: 'short', day: 'numeric', month: 'short' });
  return `${day} ${time}`;
}
