// Home Page - Main App Page

import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { formatMinutes, formatClockTime, formatTimeOnly } from '../utils/timeFormat';
import { isBlockingError } from '../api/client';
import { TimeDisplay } from '../components/TimeDisplay';
import { ActiveSession } from '../components/ActiveSession';
import { DeviceButton } from '../components/DeviceButton';
import { DurationPicker } from '../components/DurationPicker';
import { LanguageSwitcher } from '../components/LanguageSwitcher';
import { useI18n } from '../i18n/LanguageContext';

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
    refresh,
  } = useApp();
  const { t, lang } = useI18n();

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
    if (!selectedDeviceId || devices.find(d => d.id === selectedDeviceId)?.locked) return;

    try {
      setActionLoading(true);
      await createSession(selectedDeviceId, minutes);
      setSelectedDeviceId(null);
    } catch (err) {
      alert(err instanceof Error ? err.message : t('action.startFailed'));
      if (isBlockingError(err)) {
        setSelectedDeviceId(null);
        void refresh();
      }
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
      alert(err instanceof Error ? err.message : t('action.stopFailed'));
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
      alert(err instanceof Error ? err.message : t('action.extendFailed'));
      if (isBlockingError(err)) {
        void refresh();
      }
    } finally {
      setActionLoading(false);
    }
  };

  if (!child || !stats) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-2xl text-gray-600">{t('home.loading')}</div>
      </div>
    );
  }

  const hasNoTime = stats.remaining_minutes === 0;
  const isInDowntime = stats.downtime_enabled && stats.in_downtime;
  const isLockdown = !!stats.lockdown;
  const approvalRequired = !!stats.approval?.required;
  const needsApproval = approvalRequired && !stats.approval?.approved;
  const isApprovedToday = approvalRequired && !!stats.approval?.approved;
  // Any state where the child cannot start a session
  const isBlocked = isLockdown || isInDowntime || needsApproval;

  const allDevicesLocked = devices.length > 0 && devices.every(d => d.locked);
  // Ignore a selection that became locked after a refresh
  const selectedDevice = devices.find(d => d.id === selectedDeviceId && !d.locked);

  const activeDevice = activeSession ? devices.find(d => d.id === activeSession.device_id) : undefined;

  // Why extending the active session is not allowed (priority: lockdown > downtime > approval > device lock)
  let extendBlockedReason: string | null = null;
  if (isLockdown) {
    extendBlockedReason = t('extend.lockdown');
  } else if (isInDowntime) {
    extendBlockedReason = t('extend.downtime');
  } else if (needsApproval) {
    extendBlockedReason = t('extend.approval');
  } else if (activeDevice?.locked) {
    extendBlockedReason = t('extend.deviceLocked');
  }

  return (
    <div className="min-h-screen pb-8">
      {/* Header */}
      <div className="bg-white shadow-lg sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-4 py-4 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-800">
              {t('home.hi', { name: child.name })}
            </h1>
            <p className="text-sm text-gray-500">
              {t('home.subtitle')}
              {isApprovedToday && !isLockdown && (
                <span className="ml-2 inline-block text-xs font-semibold text-green-700 bg-green-50 border border-green-200 rounded-full px-2 py-0.5">
                  {t('home.allowedToday')}
                </span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <LanguageSwitcher />
            <button
              onClick={handleLogout}
              className="bg-gray-200 text-gray-700 font-semibold py-2 px-4 rounded-xl hover:bg-gray-300 transition"
            >
              {t('home.logout')}
            </button>
          </div>
        </div>
      </div>

      <div className="max-w-4xl mx-auto px-4 py-8 space-y-8">
        {/* Lockdown Notice - Show first when lockdown is active */}
        {isLockdown && (
          <div className="card" style={{
            background: 'linear-gradient(135deg, #f87171 0%, #dc2626 100%)',
            border: '3px solid #ef4444'
          }}>
            <div className="text-center py-10 px-6">
              <div className="text-7xl mb-4">🔒</div>
              <div className="text-3xl font-black mb-3 text-white">
                {t('lockdown.title')}
              </div>
              <div className="text-lg font-semibold text-white max-w-md mx-auto">
                {t('lockdown.text')}
              </div>
              {stats.lockdown_until && (
                <div className="mt-6 inline-block bg-white/20 backdrop-blur-sm rounded-2xl px-8 py-4 border-2 border-white/30">
                  <div className="text-sm font-semibold text-white/90 mb-1">{t('lockdown.until')}</div>
                  <div className="text-3xl font-black text-white">
                    {formatClockTime(stats.lockdown_until, lang)}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Downtime Notice - Show first when in downtime */}
        {!isLockdown && stats.downtime_enabled && stats.in_downtime && !activeSession && (
          <div className="card" style={{
            background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
            border: '3px solid #9f7aea'
          }}>
            <div className="text-center py-10 px-6">
              <div className="text-7xl mb-4">🌙</div>
              <div className="text-3xl font-black mb-3 text-white">
                {t('downtime.title')}
              </div>
              <div className="text-lg font-semibold text-white mb-6 max-w-md mx-auto">
                {t('downtime.text')}
              </div>
              {stats.downtime_end && (
                <div className="mt-4 inline-block bg-white/20 backdrop-blur-sm rounded-2xl px-8 py-4 border-2 border-white/30">
                  <div className="text-sm font-semibold text-white/90 mb-1">{t('downtime.ends')}</div>
                  <div className="text-3xl font-black text-white">
                    {formatTimeOnly(stats.downtime_end, lang)}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Approval Notice - school days: parents must allow screen time first */}
        {!isLockdown && !isInDowntime && needsApproval && !activeSession && !hasNoTime && (
          <div className="card bg-gradient-to-br from-amber-50 to-yellow-50 border-2 border-amber-200">
            <div className="text-center py-8 px-6">
              <div className="text-6xl mb-4">📚</div>
              <div className="text-2xl font-bold text-gray-800 mb-2">
                {t('approval.title')}
              </div>
              <div className="text-gray-600 max-w-md mx-auto">
                {t('approval.text')}
              </div>
            </div>
          </div>
        )}

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
              {t('home.activeTitle')}
            </h2>
            <ActiveSession
              session={activeSession}
              device={activeDevice}
              onStop={handleStopSession}
              onExtend={handleExtendSession}
              loading={actionLoading}
              extendBlockedReason={extendBlockedReason}
            />
          </div>
        )}

        {/* No Time Message */}
        {hasNoTime && !activeSession && !isLockdown && (
          <div className="card bg-yellow-50 border-2 border-yellow-200">
            <div className="text-center py-8">
              <div className="text-6xl mb-4">⏰</div>
              <div className="text-2xl font-bold text-gray-800 mb-2">
                {t('noTime.title')}
              </div>
              <div className="text-gray-600">
                {t('noTime.text')}
              </div>
            </div>
          </div>
        )}

        {/* All devices locked */}
        {!activeSession && !hasNoTime && !isBlocked && allDevicesLocked && (
          <div className="card bg-gray-50 border-2 border-gray-200">
            <div className="text-center py-8">
              <div className="text-6xl mb-4">🔒</div>
              <div className="text-2xl font-bold text-gray-800 mb-2">
                {t('allLocked.title')}
              </div>
              <div className="text-gray-600">
                {t('allLocked.text')}
              </div>
            </div>
          </div>
        )}

        {/* Device Selection (only if no active session, has time, and nothing blocks it) */}
        {!activeSession && !hasNoTime && !isBlocked && !allDevicesLocked && (
          <div>
            <h2 className="text-xl font-bold text-gray-800 mb-4">
              {t('home.chooseDevice')}
            </h2>
            <div className="overflow-x-auto pb-4">
              <div className="flex gap-4 px-2">
                {devices.map((device) => (
                  <DeviceButton
                    key={device.id}
                    device={device}
                    selected={selectedDevice?.id === device.id}
                    onClick={() => setSelectedDeviceId(device.id)}
                  />
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Duration Selection (only if an unlocked device is selected and nothing blocks it) */}
        {!activeSession && !hasNoTime && !isBlocked && selectedDevice && (
          <div className="card bg-gradient-to-br from-purple-50 to-pink-50">
            <DurationPicker
              onSelect={handleCreateSession}
              maxMinutes={stats.remaining_minutes}
              disabled={actionLoading}
            />
            {actionLoading && (
              <div className="text-center mt-4 text-purple-600 font-semibold">
                {t('home.starting')}
              </div>
            )}
          </div>
        )}

        {/* Stats Summary */}
        <div className="card bg-gradient-to-br from-gray-50 to-slate-50">
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <div className="text-3xl font-black text-purple-600">
                {formatMinutes(stats.used_minutes, lang)}
              </div>
              <div className="text-sm text-gray-600 font-medium">{t('home.statsUsed')}</div>
            </div>
            <div>
              <div className="text-3xl font-black text-green-600">
                {formatMinutes(stats.remaining_minutes, lang)}
              </div>
              <div className="text-sm text-gray-600 font-medium">{t('home.statsRemaining')}</div>
            </div>
            <div>
              <div className="text-3xl font-black text-blue-600">
                {stats.sessions_count}
              </div>
              <div className="text-sm text-gray-600 font-medium">{t('home.statsSessions')}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
