// Active Session Component

import { useState, useEffect } from 'react';
import type { Session, Device } from '../api/types';

interface ActiveSessionProps {
  session: Session;
  device?: Device;
  onStop: () => void;
  stopping?: boolean;
}

export function ActiveSession({ session, device, onStop, stopping }: ActiveSessionProps) {
  const [localRemaining, setLocalRemaining] = useState(session.remaining_minutes);

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

  return (
    <div className="card bg-gradient-to-br from-purple-500 to-pink-500 text-white p-6">
      <div className="flex flex-col gap-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="text-4xl">{getDeviceEmoji(session.device_type)}</div>
            <div>
              <div className="text-sm opacity-90">Currently playing on</div>
              <div className="text-xl font-bold">{device?.name || session.device_id}</div>
            </div>
          </div>
        </div>

        {/* Time remaining */}
        <div className="bg-white/20 rounded-2xl p-4 text-center">
          <div className="text-3xl font-bold">{localRemaining} min</div>
          <div className="text-sm opacity-90">remaining</div>
        </div>

        {/* Stop button */}
        <button
          onClick={onStop}
          disabled={stopping}
          className="bg-white text-purple-600 font-bold py-3 px-6 rounded-2xl shadow-lg transform transition hover:scale-105 active:scale-95 disabled:opacity-50 disabled:hover:scale-100"
        >
          {stopping ? 'Stopping...' : 'Stop Session'}
        </button>
      </div>
    </div>
  );
}
