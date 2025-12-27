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
} from './types';

const API_BASE_URL = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const SESSION_KEY = 'metron_child_session';

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
      throw new Error(error.error || 'Network error');
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

  async createSession(deviceId: string, minutes: number, shared?: boolean): Promise<Session> {
    const request: CreateSessionRequest = {
      device_id: deviceId,
      minutes,
      shared: shared || false,
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
