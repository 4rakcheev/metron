// Device Selection Button Component

import type { Device } from '../api/types';

interface DeviceButtonProps {
  device: Device;
  selected: boolean;
  onClick: () => void;
}

export function DeviceButton({ device, selected, onClick }: DeviceButtonProps) {
  // Get device emoji based on type
  const getDeviceEmoji = (type: string): string => {
    const emojiMap: Record<string, string> = {
      tv: '📺',
      ipad: '📱',
      ps5: '🎮',
      xbox: '🎮',
      computer: '💻',
      tablet: '📱',
    };
    return emojiMap[type.toLowerCase()] || '🖥️';
  };

  // Get color gradient based on type
  const getGradient = (type: string): string => {
    const gradientMap: Record<string, string> = {
      tv: 'from-blue-500 to-cyan-500',
      ipad: 'from-purple-500 to-pink-500',
      ps5: 'from-indigo-500 to-purple-500',
      xbox: 'from-green-500 to-emerald-500',
      computer: 'from-orange-500 to-red-500',
      tablet: 'from-pink-500 to-rose-500',
    };
    return gradientMap[type.toLowerCase()] || 'from-gray-500 to-gray-600';
  };

  return (
    <button
      onClick={onClick}
      className={`
        flex-shrink-0 w-32 h-32 rounded-3xl shadow-lg transform transition-all
        ${selected ? 'scale-110 shadow-2xl ring-4 ring-white' : 'scale-100 hover:scale-105'}
        active:scale-95
        bg-gradient-to-br ${getGradient(device.type)}
        flex flex-col items-center justify-center gap-2 text-white
      `}
    >
      <div className="text-4xl">{getDeviceEmoji(device.type)}</div>
      <div className="text-sm font-bold text-center px-2">{device.name}</div>
    </button>
  );
}
