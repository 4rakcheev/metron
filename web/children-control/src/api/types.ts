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
  // True only when a parent lockdown blocks this child on ALL devices
  lockdown?: boolean;
  // RFC3339; absent means "until parents unlock"
  lockdown_until?: string;
  // Present only when the server has the daily approval gate enabled
  approval?: ApprovalStatus;
}

export interface ApprovalStatus {
  // True on days when a parent must allow screen time first (school days)
  required: boolean;
  approved: boolean;
  approved_at: string | null;
}

export interface Device {
  id: string;
  name: string;
  type: string;
  emoji?: string;
  // The parent blocked this device for this child
  locked?: boolean;
  // RFC3339; absent means "until parents unlock"
  locked_until?: string;
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

export type APIErrorCode =
  | 'LOCKDOWN_ACTIVE'
  | 'DOWNTIME_ACTIVE'
  | 'APPROVAL_REQUIRED'
  | 'INSUFFICIENT_TIME'
  | 'NETWORK_ERROR'
  | (string & {});

export interface APIError {
  error: string;
  code: APIErrorCode;
  details?: string;
}
