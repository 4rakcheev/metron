import type { Device } from '../api/types';

const emojiMap: Record<string, string> = {
  tv: '📺',
  ipad: '⬛',
  tablet: '⬛',
  phone: '📱',
  iphone: '📱',
  ps5: '🎮',
  xbox: '🎮',
  computer: '💻',
};

export function getDeviceEmoji(type: string): string {
  return emojiMap[type.toLowerCase()] || '🖥️';
}

export function resolveDeviceEmoji(device: Device | undefined, fallbackType: string): string {
  return device?.emoji || getDeviceEmoji(device?.type ?? fallbackType);
}
