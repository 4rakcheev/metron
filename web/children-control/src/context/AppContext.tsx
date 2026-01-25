// Global Application Context for State Management

import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { api } from '../api/client';
import type { Child, TodayStats, Device, Session, MovieTimeAvailability } from '../api/types';

interface AppState {
  child: Child | null;
  stats: TodayStats | null;
  devices: Device[];
  sessions: Session[];
  movieTime: MovieTimeAvailability | null;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
}

interface AppContextValue extends AppState {
  login: (childId: string, pin: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  createSession: (deviceId: string, minutes: number) => Promise<void>;
  stopSession: (sessionId: string) => Promise<void>;
  extendSession: (sessionId: string, additionalMinutes: number) => Promise<void>;
  startMovieTime: (deviceId: string) => Promise<void>;
  clearError: () => void;
}

const AppContext = createContext<AppContextValue | undefined>(undefined);

const REFRESH_INTERVAL = 30000; // 30 seconds

export function AppProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AppState>({
    child: null,
    stats: null,
    devices: [],
    sessions: [],
    movieTime: null,
    loading: false,
    error: null,
    isAuthenticated: api.isAuthenticated(),
  });

  // Load data function
  const loadData = useCallback(async () => {
    if (!api.isAuthenticated()) {
      setState(prev => ({
        ...prev,
        isAuthenticated: false,
        child: null,
        stats: null,
        devices: [],
        sessions: [],
        movieTime: null,
      }));
      return;
    }

    try {
      setState(prev => ({ ...prev, loading: true, error: null }));

      // Load all data in parallel
      const [child, stats, devices, sessions, movieTime] = await Promise.all([
        api.getMe(),
        api.getToday(),
        api.getDevices(),
        api.getSessions(),
        api.getMovieTimeAvailability(),
      ]);

      setState(prev => ({
        ...prev,
        child,
        stats,
        devices,
        sessions,
        movieTime,
        loading: false,
        isAuthenticated: true,
      }));
    } catch (err) {
      console.error('Failed to load data:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to load data',
        isAuthenticated: false,
      }));
    }
  }, []);

  // Login function
  const login = useCallback(async (childId: string, pin: string) => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }));
      const response = await api.login(childId, pin);

      setState(prev => ({
        ...prev,
        child: response.child,
        isAuthenticated: true,
        loading: false,
      }));

      // Load full data after login
      await loadData();
    } catch (err) {
      console.error('Login failed:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Login failed',
      }));
      throw err;
    }
  }, [loadData]);

  // Logout function
  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch (err) {
      console.error('Logout error:', err);
    } finally {
      setState({
        child: null,
        stats: null,
        devices: [],
        sessions: [],
        movieTime: null,
        loading: false,
        error: null,
        isAuthenticated: false,
      });
    }
  }, []);

  // Create session function
  const createSession = useCallback(async (deviceId: string, minutes: number) => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }));
      await api.createSession(deviceId, minutes);

      // Reload data to get updated sessions and stats
      await loadData();
    } catch (err) {
      console.error('Failed to create session:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to create session',
      }));
      throw err;
    }
  }, [loadData]);

  // Stop session function
  const stopSession = useCallback(async (sessionId: string) => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }));
      await api.stopSession(sessionId);

      // Reload data to get updated sessions and stats
      await loadData();
    } catch (err) {
      console.error('Failed to stop session:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to stop session',
      }));
      throw err;
    }
  }, [loadData]);

  // Extend session function
  const extendSession = useCallback(async (sessionId: string, additionalMinutes: number) => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }));
      await api.extendSession(sessionId, additionalMinutes);

      // Reload data to get updated sessions and stats
      await loadData();
    } catch (err) {
      console.error('Failed to extend session:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to extend session',
      }));
      throw err;
    }
  }, [loadData]);

  // Start movie time function
  const startMovieTime = useCallback(async (deviceId: string) => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }));
      await api.startMovieTime(deviceId);

      // Reload data to get updated sessions and movie time status
      await loadData();
    } catch (err) {
      console.error('Failed to start movie time:', err);
      setState(prev => ({
        ...prev,
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to start movie time',
      }));
      throw err;
    }
  }, [loadData]);

  // Clear error function
  const clearError = useCallback(() => {
    setState(prev => ({ ...prev, error: null }));
  }, []);

  // Auto-refresh effect
  useEffect(() => {
    if (state.isAuthenticated) {
      loadData();

      const interval = setInterval(() => {
        loadData();
      }, REFRESH_INTERVAL);

      return () => clearInterval(interval);
    }
  }, [state.isAuthenticated, loadData]);

  const value: AppContextValue = {
    ...state,
    login,
    logout,
    refresh: loadData,
    createSession,
    stopSession,
    extendSession,
    startMovieTime,
    clearError,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

// Custom hook to use the app context
export function useApp(): AppContextValue {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error('useApp must be used within AppProvider');
  }
  return context;
}
