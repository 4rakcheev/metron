// Duration Picker Component

interface DurationPickerProps {
  onSelect: (minutes: number) => void;
  maxMinutes: number;
  disabled?: boolean;
}

interface DurationOption {
  minutes: number;
  label: string;
  gradient: string;
}

const durationOptions: DurationOption[] = [
  { minutes: 5, label: '5 min', gradient: 'from-green-400 to-emerald-500' },
  { minutes: 15, label: '15 min', gradient: 'from-blue-400 to-cyan-500' },
  { minutes: 30, label: '30 min', gradient: 'from-purple-400 to-pink-500' },
  { minutes: 60, label: '1 hour', gradient: 'from-orange-400 to-red-500' },
];

export function DurationPicker({ onSelect, maxMinutes, disabled }: DurationPickerProps) {
  const handleSelect = (requestedMinutes: number) => {
    // If there's any time available, allocate up to the requested amount
    const minutesToAllocate = Math.min(requestedMinutes, maxMinutes);
    onSelect(minutesToAllocate);
  };

  return (
    <div className="w-full">
      <div className="text-center text-gray-800 font-bold mb-4 text-lg">
        How long do you want to play?
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
              {option.label}
              {willGetLess && (
                <div className="text-xs mt-1">
                  ({maxMinutes} min available)
                </div>
              )}
              {hasNoTime && (
                <div className="text-xs mt-1">No time left</div>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
