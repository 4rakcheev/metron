// API Types for Metron Child App

export interface Child {
  id: string;
  name: string;
  emoji: string;
  weekday_limit: number;
  weekend_limit: number;
}

export interface ChildForAuth {
  id: string;
  name: string;
  emoji: string;
}

export interface TodayStats {
  used_minutes: number;
  remaining_minutes: number;
  daily_limit: number;
  sessions_count: number;
  downtime_enabled: boolean;
  in_downtime: boolean;
  downtime_end?: string;
}

export interface Device {
  id: string;
  name: string;
  type: string;
}

export interface Session {
  id: string;
  device_id: string;
  device_type: string;
  start_time: string;
  remaining_minutes: number;
  status: string;
}

export interface LoginRequest {
  child_id: string;
  pin: string;
}

export interface LoginResponse {
  session_id: string;
  child: Child;
}

export interface CreateSessionRequest {
  device_id: string;
  minutes: number;
}

export interface APIError {
  error: string;
  code: string;
  details?: string;
}

export interface MovieTimeAvailability {
  is_weekend: boolean;
  is_bypass_active: boolean;
  bypass_reason?: string;
  is_available: boolean;
  is_used_today: boolean;
  break_required: boolean;
  break_minutes_left: number;
  last_session_end?: string;
  can_start: boolean;
  reason?: string;
  allowed_devices: string[];
  duration_minutes: number;
}

export interface StartMovieTimeRequest {
  device_id: string;
}
