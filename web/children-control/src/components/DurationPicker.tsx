// Duration Picker Component

import { useI18n } from '../i18n/LanguageContext';
import type { MessageKey } from '../i18n';

interface DurationPickerProps {
  onSelect: (minutes: number) => void;
  maxMinutes: number;
  disabled?: boolean;
}

interface DurationOption {
  minutes: number;
  labelKey: MessageKey;
  gradient: string;
}

const durationOptions: DurationOption[] = [
  { minutes: 5, labelKey: 'duration.minutes', gradient: 'from-green-400 to-emerald-500' },
  { minutes: 15, labelKey: 'duration.minutes', gradient: 'from-blue-400 to-cyan-500' },
  { minutes: 30, labelKey: 'duration.minutes', gradient: 'from-purple-400 to-pink-500' },
  { minutes: 60, labelKey: 'duration.hour', gradient: 'from-orange-400 to-red-500' },
];

export function DurationPicker({ onSelect, maxMinutes, disabled }: DurationPickerProps) {
  const { t } = useI18n();

  const handleSelect = (requestedMinutes: number) => {
    // If there's any time available, allocate up to the requested amount
    const minutesToAllocate = Math.min(requestedMinutes, maxMinutes);
    onSelect(minutesToAllocate);
  };

  return (
    <div className="w-full">
      <div className="text-center text-gray-800 font-bold mb-4 text-lg">
        {t('duration.title')}
      </div>

      <div className="grid grid-cols-2 gap-4 max-w-md mx-auto">
        {durationOptions.map((option) => {
          // Only disable if there's absolutely no time left
          const hasNoTime = maxMinutes === 0;
          const willGetLess = option.minutes > maxMinutes && maxMinutes > 0;

          return (
            <button
              key={option.minutes}
              onClick={() => handleSelect(option.minutes)}
              disabled={disabled || hasNoTime}
              className={`
                h-24 rounded-2xl shadow-lg font-bold text-xl
                transform transition-all
                ${
                  !hasNoTime
                    ? `bg-gradient-to-br ${option.gradient} text-white hover:scale-105 active:scale-95`
                    : 'bg-gray-200 text-gray-700 cursor-not-allowed border-2 border-gray-300'
                }
                ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
              `}
            >
              {t(option.labelKey, { n: option.minutes })}
              {willGetLess && (
                <div className="text-xs mt-1">
                  {t('duration.available', { n: maxMinutes })}
                </div>
              )}
              {hasNoTime && (
                <div className="text-xs mt-1">{t('duration.noTime')}</div>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
