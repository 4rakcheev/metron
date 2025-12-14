// Active Session Component

import { useState, useEffect } from 'react';
import { formatMinutes } from '../utils/timeFormat';
import type { Session, Device } from '../api/types';

interface ActiveSessionProps {
  session: Session;
  device?: Device;
  onStop: () => void;
  onExtend: (minutes: number) => void;
  loading?: boolean;
}

export function ActiveSession({ session, device, onStop, onExtend, loading }: ActiveSessionProps) {
  const [localRemaining, setLocalRemaining] = useState(session.remaining_minutes);
  const [showExtendOptions, setShowExtendOptions] = useState(false);

  // Local countdown timer that updates every minute
  useEffect(() => {
    setLocalRemaining(session.remaining_minutes);

    const interval = setInterval(() => {
      setLocalRemaining(prev => Math.max(0, prev - 1));
    }, 60000); // Update every minute

    return () => clearInterval(interval);
  }, [session.remaining_minutes]);

  // Get device emoji based on type
  const getDeviceEmoji = (type: string): string => {
    const emojiMap: Record<string, string> = {
      tv: '📺',
      ipad: '📱',
      ps5: '🎮',
      xbox: '🎮',
      computer: '💻',
    };
    return emojiMap[type.toLowerCase()] || '🖥️';
  };

  const extendOptions = [5, 15, 30, 60];

  const handleExtend = (minutes: number) => {
    setShowExtendOptions(false);
    onExtend(minutes);
  };

  return (
    <div className="card bg-gradient-to-br from-purple-500 to-pink-500 text-white p-6 shadow-xl">
      <div className="flex flex-col gap-5">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="text-5xl">{getDeviceEmoji(session.device_type)}</div>
            <div>
              <div className="text-sm opacity-90">Currently playing on</div>
              <div className="text-2xl font-bold">{device?.name || session.device_id}</div>
            </div>
          </div>
        </div>

        {/* Time remaining - BIG AND VISIBLE */}
        <div className="bg-white/25 backdrop-blur-sm rounded-3xl p-6 text-center border-2 border-white/30">
          <div className="text-6xl font-black tracking-tight mb-2">{formatMinutes(localRemaining)}</div>
          <div className="text-lg font-semibold opacity-90">remaining</div>
        </div>

        {/* Action buttons */}
        {!showExtendOptions ? (
          <div className="grid grid-cols-2 gap-3">
            <button
              onClick={() => setShowExtendOptions(true)}
              disabled={loading}
              className="bg-white/20 backdrop-blur-sm text-white font-bold py-4 px-6 rounded-2xl border-2 border-white/30 shadow-lg transform transition hover:scale-105 active:scale-95 disabled:opacity-50 disabled:hover:scale-100"
            >
              ⏱️ Extend
            </button>
            <button
              onClick={onStop}
              disabled={loading}
              className="bg-white text-purple-600 font-bold py-4 px-6 rounded-2xl shadow-lg transform transition hover:scale-105 active:scale-95 disabled:opacity-50 disabled:hover:scale-100"
            >
              {loading ? '...' : '🛑 Stop'}
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="text-center text-sm font-semibold opacity-90">
              How much more time?
            </div>
            <div className="grid grid-cols-2 gap-2">
              {extendOptions.map((mins) => (
                <button
                  key={mins}
                  onClick={() => handleExtend(mins)}
                  disabled={loading}
                  className="bg-white/20 backdrop-blur-sm text-white font-bold py-3 px-4 rounded-xl border-2 border-white/30 shadow transform transition hover:scale-105 active:scale-95 disabled:opacity-50"
                >
                  +{mins} min
                </button>
              ))}
            </div>
            <button
              onClick={() => setShowExtendOptions(false)}
              className="w-full bg-white/10 backdrop-blur-sm text-white font-semibold py-2 px-4 rounded-xl border border-white/20 hover:bg-white/20 transition"
            >
              Cancel
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
