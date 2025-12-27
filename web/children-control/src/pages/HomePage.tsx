// Home Page - Main App Page

import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { formatMinutes } from '../utils/timeFormat';
import { TimeDisplay } from '../components/TimeDisplay';
import { ActiveSession } from '../components/ActiveSession';
import { DeviceButton } from '../components/DeviceButton';
import { DurationPicker } from '../components/DurationPicker';

export function HomePage() {
  const navigate = useNavigate();
  const {
    isAuthenticated,
    child,
    stats,
    devices,
    sessions,
    logout,
    createSession,
    stopSession,
    extendSession,
  } = useApp();

  const [selectedDeviceId, setSelectedDeviceId] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // Redirect if not authenticated
  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login');
    }
  }, [isAuthenticated, navigate]);

  // Get active session for this child (if any)
  const activeSession = sessions.find(s => s.status === 'active');

  // Handle logout
  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  // Handle session creation
  const handleCreateSession = async (minutes: number) => {
    if (!selectedDeviceId) return;

    try {
      setActionLoading(true);
      await createSession(selectedDeviceId, minutes);
      setSelectedDeviceId(null);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to start session');
    } finally {
      setActionLoading(false);
    }
  };

  // Handle session stop
  const handleStopSession = async () => {
    if (!activeSession) return;

    try {
      setActionLoading(true);
      await stopSession(activeSession.id);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to stop session');
    } finally {
      setActionLoading(false);
    }
  };

  // Handle session extend
  const handleExtendSession = async (minutes: number) => {
    if (!activeSession) return;

    try {
      setActionLoading(true);
      await extendSession(activeSession.id, minutes);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to extend session');
    } finally {
      setActionLoading(false);
    }
  };

  if (!child || !stats) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-2xl text-gray-600">Loading...</div>
      </div>
    );
  }

  const hasNoTime = stats.remaining_minutes === 0;
  const isInDowntime = stats.downtime_enabled && stats.in_downtime;

  return (
    <div className="min-h-screen pb-8">
      {/* Header */}
      <div className="bg-white shadow-lg sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-4 py-4 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-800">
              Hi, {child.name}! 👋
            </h1>
            <p className="text-sm text-gray-500">Ready to have fun?</p>
          </div>
          <button
            onClick={handleLogout}
            className="bg-gray-200 text-gray-700 font-semibold py-2 px-4 rounded-xl hover:bg-gray-300 transition"
          >
            Logout
          </button>
        </div>
      </div>

      <div className="max-w-4xl mx-auto px-4 py-8 space-y-8">
        {/* Time Display */}
        <div className="card">
          <TimeDisplay
            remainingMinutes={stats.remaining_minutes}
            totalMinutes={stats.daily_limit}
          />
        </div>

        {/* Active Session Block */}
        {activeSession && (
          <div>
            <h2 className="text-xl font-bold text-gray-800 mb-4">
              🎮 Active Session
            </h2>
            <ActiveSession
              session={activeSession}
              device={devices.find(d => d.id === activeSession.device_id)}
              onStop={handleStopSession}
              onExtend={handleExtendSession}
              loading={actionLoading}
            />
          </div>
        )}

        {/* Downtime Notice */}
        {stats.downtime_enabled && stats.in_downtime && !activeSession && (
          <div className="card bg-purple-900 text-white">
            <div className="text-center py-8">
              <div className="text-6xl mb-4">🌙</div>
              <div className="text-2xl font-bold mb-2">
                Downtime Period
              </div>
              <div className="text-purple-200">
                You cannot start or extend sessions during downtime.
              </div>
              {stats.downtime_end && (
                <div className="mt-4 text-sm text-purple-300">
                  Downtime ends at {new Date(stats.downtime_end).toLocaleTimeString()}
                </div>
              )}
            </div>
          </div>
        )}

        {/* No Time Message */}
        {hasNoTime && !activeSession && (
          <div className="card bg-yellow-50 border-2 border-yellow-200">
            <div className="text-center py-8">
              <div className="text-6xl mb-4">⏰</div>
              <div className="text-2xl font-bold text-gray-800 mb-2">
                No time left today
              </div>
              <div className="text-gray-600">
                You've used all your screen time for today. Come back tomorrow!
              </div>
            </div>
          </div>
        )}

        {/* Device Selection (only if no active session, has time, and not in downtime) */}
        {!activeSession && !hasNoTime && !isInDowntime && (
          <div>
            <h2 className="text-xl font-bold text-gray-800 mb-4">
              📱 Choose a device
            </h2>
            <div className="overflow-x-auto pb-4">
              <div className="flex gap-4 px-2">
                {devices.map((device) => (
                  <DeviceButton
                    key={device.id}
                    device={device}
                    selected={selectedDeviceId === device.id}
                    onClick={() => setSelectedDeviceId(device.id)}
                  />
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Duration Selection (only if device selected and not in downtime) */}
        {!activeSession && !hasNoTime && !isInDowntime && selectedDeviceId && (
          <div className="card bg-gradient-to-br from-purple-50 to-pink-50">
            <DurationPicker
              onSelect={handleCreateSession}
              maxMinutes={stats.remaining_minutes}
              disabled={actionLoading}
            />
            {actionLoading && (
              <div className="text-center mt-4 text-purple-600 font-semibold">
                Starting session...
              </div>
            )}
          </div>
        )}

        {/* Stats Summary */}
        <div className="card bg-gradient-to-br from-gray-50 to-slate-50">
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <div className="text-3xl font-black text-purple-600">
                {formatMinutes(stats.used_minutes)}
              </div>
              <div className="text-sm text-gray-600 font-medium">Used</div>
            </div>
            <div>
              <div className="text-3xl font-black text-green-600">
                {formatMinutes(stats.remaining_minutes)}
              </div>
              <div className="text-sm text-gray-600 font-medium">Remaining</div>
            </div>
            <div>
              <div className="text-3xl font-black text-blue-600">
                {stats.sessions_count}
              </div>
              <div className="text-sm text-gray-600 font-medium">Sessions</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
