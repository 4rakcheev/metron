// Device Selection Button Component

import type { Device } from '../api/types';
import { resolveDeviceEmoji } from '../utils/deviceEmoji';
import { formatClockTime } from '../utils/timeFormat';
import { useI18n } from '../i18n/LanguageContext';

interface DeviceButtonProps {
  device: Device;
  selected: boolean;
  onClick: () => void;
}

export function DeviceButton({ device, selected, onClick }: DeviceButtonProps) {
  const { t, lang } = useI18n();

  // Get color gradient based on type
  const getGradient = (type: string): string => {
    const gradientMap: Record<string, string> = {
      tv: 'from-blue-500 to-cyan-500',
      ipad: 'from-purple-500 to-pink-500',
      tablet: 'from-pink-500 to-rose-500',
      phone: 'from-teal-500 to-cyan-500',
      iphone: 'from-teal-500 to-cyan-500',
      ps5: 'from-indigo-500 to-purple-500',
      game: 'from-indigo-500 to-purple-500',
      xbox: 'from-green-500 to-emerald-500',
      computer: 'from-orange-500 to-red-500',
    };
    return gradientMap[type.toLowerCase()] || 'from-gray-500 to-gray-600';
  };

  const locked = !!device.locked;

  return (
    <button
      onClick={locked ? undefined : onClick}
      disabled={locked}
      aria-disabled={locked}
      title={locked ? t('device.lockedHint') : undefined}
      className={`
        relative flex-shrink-0 w-32 h-32 rounded-3xl shadow-lg transform transition-all
        ${locked
          ? 'bg-gray-300 text-gray-500 cursor-not-allowed opacity-80'
          : `${selected ? 'scale-110 shadow-2xl ring-4 ring-white' : 'scale-100 hover:scale-105'}
             active:scale-95 bg-gradient-to-br ${getGradient(device.type)} text-white`}
        flex flex-col items-center justify-center gap-1
      `}
    >
      {locked && (
        <div className="absolute top-2 right-2 text-lg" aria-hidden="true">🔒</div>
      )}
      <div className={`text-4xl ${locked ? 'grayscale' : ''}`}>{resolveDeviceEmoji(device, device.type)}</div>
      <div className="text-sm font-bold text-center px-2">{device.name}</div>
      {locked && (
        <div className="text-xs font-semibold text-center px-2">
          {device.locked_until
            ? t('device.lockedUntil', { time: formatClockTime(device.locked_until, lang) })
            : t('device.locked')}
        </div>
      )}
    </button>
  );
}
