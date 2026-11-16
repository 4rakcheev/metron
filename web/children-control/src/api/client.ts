// API Client for Metron Child App

import type {
  Child,
  ChildForAuth,
  TodayStats,
  Device,
  Session,
  LoginRequest,
  LoginResponse,
  CreateSessionRequest,
  APIError,
  APIErrorCode,
} from './types';

const API_BASE_URL = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const SESSION_KEY = 'metron_child_session';

// Child-friendly messages for known error codes (fall back to the server text)
const FRIENDLY_ERROR_MESSAGES: Record<string, string> = {
  LOCKDOWN_ACTIVE: '🔒 Your parents have locked screen time right now.',
  DOWNTIME_ACTIVE: '🌙 It is rest time now. Screen time is not available.',
  APPROVAL_REQUIRED: '📚 Ask your parents to allow screen time for today.',
  INSUFFICIENT_TIME: '⏰ Not enough time left today.',
};

// Codes that mean the screen state changed and the UI should refresh
const BLOCKING_ERROR_CODES = new Set<string>([
  'LOCKDOWN_ACTIVE',
  'DOWNTIME_ACTIVE',
  'APPROVAL_REQUIRED',
  'INSUFFICIENT_TIME',
]);

export class APIRequestError extends Error {
  readonly code: APIErrorCode;
  readonly status: number;

  constructor(message: string, code: APIErrorCode, status: number) {
    super(message);
    this.name = 'APIRequestError';
    this.code = code;
    this.status = status;
  }
}

// True when the error means a block (lockdown, downtime, approval, no time)
export function isBlockingError(err: unknown): boolean {
  return err instanceof APIRequestError && BLOCKING_ERROR_CODES.has(err.code);
}

class MetronAPI {
  private sessionId: string | null = null;

  constructor() {
    // Load session from localStorage on init
    this.sessionId = localStorage.getItem(SESSION_KEY);
  }

  // Helper method to make authenticated requests
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers = new Headers(options.headers);
    headers.set('Content-Type', 'application/json');

    // Add session ID to request if available
    if (this.sessionId) {
      headers.set('Authorization', `Bearer ${this.sessionId}`);
    }

    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
      credentials: 'include', // Include cookies
    });

    // Handle 401 errors by clearing session
    if (response.status === 401) {
      this.clearSession();
      throw new Error('Session expired. Please login again.');
    }

    // Handle non-2xx responses
    if (!response.ok) {
      const error: APIError = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
        code: 'NETWORK_ERROR',
      }));
      const message = FRIENDLY_ERROR_MESSAGES[error.code] || error.error || 'Network error';
      throw new APIRequestError(message, error.code, response.status);
    }

    // Handle 204 No Content
    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Authentication methods

  async listChildren(): Promise<ChildForAuth[]> {
    return this.request<ChildForAuth[]>('/child/auth/children');
  }

  async login(childId: string, pin: string): Promise<LoginResponse> {
    const request: LoginRequest = { child_id: childId, pin };
    const response = await this.request<LoginResponse>('/child/auth/login', {
      method: 'POST',
      body: JSON.stringify(request),
    });

    // Store session ID
    this.sessionId = response.session_id;
    localStorage.setItem(SESSION_KEY, response.session_id);

    return response;
  }

  async logout(): Promise<void> {
    try {
      await this.request<void>('/child/auth/logout', {
        method: 'POST',
      });
    } finally {
      this.clearSession();
    }
  }

  clearSession(): void {
    this.sessionId = null;
    localStorage.removeItem(SESSION_KEY);
  }

  isAuthenticated(): boolean {
    return this.sessionId !== null;
  }

  // Protected methods (require authentication)

  async getMe(): Promise<Child> {
    return this.request<Child>('/child/me');
  }

  async getToday(): Promise<TodayStats> {
    return this.request<TodayStats>('/child/today');
  }

  async getDevices(): Promise<Device[]> {
    return this.request<Device[]>('/child/devices');
  }

  async getSessions(): Promise<Session[]> {
    return this.request<Session[]>('/child/sessions');
  }

  async createSession(deviceId: string, minutes: number): Promise<Session> {
    const request: CreateSessionRequest = {
      device_id: deviceId,
      minutes,
    };
    return this.request<Session>('/child/sessions', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }

  async stopSession(sessionId: string): Promise<void> {
    return this.request<void>(`/child/sessions/${sessionId}/stop`, {
      method: 'POST',
    });
  }

  async extendSession(sessionId: string, additionalMinutes: number): Promise<Session> {
    return this.request<Session>(`/child/sessions/${sessionId}/extend`, {
      method: 'POST',
      body: JSON.stringify({ additional_minutes: additionalMinutes }),
    });
  }
}

// Export singleton instance
export const api = new MetronAPI();
