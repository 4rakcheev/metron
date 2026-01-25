// Movie Time Card Component for Weekend Shared Movie Time
// This component shows movie time availability and allows starting a movie session

import { useState } from 'react';
import type { MovieTimeAvailability, Device } from '../api/types';

interface MovieTimeCardProps {
  movieTime: MovieTimeAvailability;
  devices: Device[];
  onStart: (deviceId: string) => Promise<void>;
  loading: boolean;
}

export function MovieTimeCard({ movieTime, devices, onStart, loading }: MovieTimeCardProps) {
  const [selectedDevice, setSelectedDevice] = useState<string>(
    movieTime.allowed_devices.length > 0 ? movieTime.allowed_devices[0] : ''
  );

  // Filter devices to only show allowed ones
  const allowedDevices = devices.filter(d => movieTime.allowed_devices.includes(d.id));

  // Get status badge styling
  const getStatusBadge = () => {
    if (movieTime.is_used_today) {
      return {
        text: 'Used Today',
        className: 'bg-gray-100 text-gray-600',
      };
    }
    if (movieTime.break_required) {
      return {
        text: `Break: ${movieTime.break_minutes_left}min`,
        className: 'bg-amber-100 text-amber-700',
      };
    }
    if (movieTime.can_start) {
      return {
        text: 'Available',
        className: 'bg-green-100 text-green-700',
      };
    }
    return {
      text: 'Not Available',
      className: 'bg-gray-100 text-gray-600',
    };
  };

  const status = getStatusBadge();

  const handleStart = async () => {
    if (selectedDevice && movieTime.can_start) {
      await onStart(selectedDevice);
    }
  };

  // Format duration for display
  const formatDuration = (minutes: number): string => {
    const hours = Math.floor(minutes / 60);
    const mins = minutes % 60;
    if (hours > 0 && mins > 0) {
      return `${hours}h ${mins}min`;
    }
    if (hours > 0) {
      return `${hours} hour${hours > 1 ? 's' : ''}`;
    }
    return `${mins} minutes`;
  };

  return (
    <div className="bg-gradient-to-br from-indigo-500 to-purple-600 rounded-3xl p-6 shadow-xl text-white">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className="text-4xl">🎬</div>
          <div>
            <h3 className="text-xl font-bold">Movie Time</h3>
            <p className="text-sm text-indigo-100 opacity-80">
              {movieTime.is_bypass_active
                ? `Special: ${movieTime.bypass_reason || 'Holiday mode'}`
                : 'Weekend shared session'}
            </p>
          </div>
        </div>
        <span className={`px-3 py-1 rounded-full text-sm font-medium ${status.className}`}>
          {status.text}
        </span>
      </div>

      {/* Info */}
      <div className="bg-white/10 rounded-2xl p-4 mb-4">
        <div className="flex items-center justify-between">
          <span className="text-indigo-100">Duration</span>
          <span className="font-semibold">{formatDuration(movieTime.duration_minutes)}</span>
        </div>
        <div className="text-xs text-indigo-200 mt-2">
          Does not count against personal screen time
        </div>
      </div>

      {/* Break countdown timer */}
      {movieTime.break_required && movieTime.break_minutes_left > 0 && (
        <div className="bg-amber-500/20 border border-amber-400/30 rounded-2xl p-4 mb-4">
          <div className="flex items-center gap-2 text-amber-100">
            <span className="text-xl">⏳</span>
            <span>Break required after last session</span>
          </div>
          <div className="text-2xl font-bold mt-2">
            {movieTime.break_minutes_left} minutes left
          </div>
        </div>
      )}

      {/* Used today message */}
      {movieTime.is_used_today && (
        <div className="bg-white/10 rounded-2xl p-4 mb-4 text-center">
          <span className="text-indigo-100">
            Movie time already used today. Try again tomorrow!
          </span>
        </div>
      )}

      {/* Device selector and start button */}
      {movieTime.can_start && (
        <>
          {/* Device selector */}
          {allowedDevices.length > 1 && (
            <div className="mb-4">
              <label className="text-sm text-indigo-100 mb-2 block">Select device:</label>
              <select
                value={selectedDevice}
                onChange={(e) => setSelectedDevice(e.target.value)}
                className="w-full p-3 rounded-xl bg-white/20 text-white border border-white/20
                         focus:ring-2 focus:ring-white/50 focus:outline-none"
              >
                {allowedDevices.map(device => (
                  <option key={device.id} value={device.id} className="text-gray-800">
                    {device.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Start button */}
          <button
            onClick={handleStart}
            disabled={loading || !selectedDevice}
            className={`
              w-full py-4 rounded-2xl font-bold text-lg
              transition-all transform
              ${loading || !selectedDevice
                ? 'bg-white/30 text-white/50 cursor-not-allowed'
                : 'bg-white text-indigo-600 hover:scale-[1.02] active:scale-[0.98] shadow-lg'
              }
            `}
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24">
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                    fill="none"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  />
                </svg>
                Starting...
              </span>
            ) : (
              <span>Start Movie Time 🍿</span>
            )}
          </button>
        </>
      )}

      {/* Not available reason */}
      {!movieTime.can_start && movieTime.reason && !movieTime.is_used_today && !movieTime.break_required && (
        <div className="text-center text-indigo-200 text-sm">
          {movieTime.reason}
        </div>
      )}
    </div>
  );
}
