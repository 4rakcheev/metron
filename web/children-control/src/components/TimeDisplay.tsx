// Time Display Component with Circular Progress

import { formatMinutes, formatMinutesDetailed } from '../utils/timeFormat';

interface TimeDisplayProps {
  remainingMinutes: number;
  totalMinutes: number;
}

export function TimeDisplay({ remainingMinutes, totalMinutes }: TimeDisplayProps) {
  const percentage = totalMinutes > 0 ? (remainingMinutes / totalMinutes) * 100 : 0;
  const radius = 90;
  const circumference = 2 * Math.PI * radius;
  const strokeDashoffset = circumference - (percentage / 100) * circumference;

  const timeFormatted = formatMinutesDetailed(remainingMinutes);

  // Color based on remaining time
  const getColor = (): string => {
    if (percentage > 50) return 'text-green-500';
    if (percentage > 20) return 'text-yellow-500';
    return 'text-red-500';
  };

  const getStrokeColor = (): string => {
    if (percentage > 50) return '#10b981'; // green-500
    if (percentage > 20) return '#eab308'; // yellow-500
    return '#ef4444'; // red-500
  };

  const getBgGradient = (): string => {
    if (percentage > 50) return 'from-green-50 to-emerald-50';
    if (percentage > 20) return 'from-yellow-50 to-amber-50';
    return 'from-red-50 to-orange-50';
  };

  return (
    <div className={`flex flex-col items-center gap-6 p-8 bg-gradient-to-br ${getBgGradient()} rounded-3xl shadow-lg`}>
      <div className="relative w-56 h-56">
        {/* Background circle */}
        <svg className="transform -rotate-90 w-56 h-56">
          <circle
            cx="112"
            cy="112"
            r={radius}
            stroke="#e5e7eb"
            strokeWidth="16"
            fill="none"
          />
          {/* Progress circle */}
          <circle
            cx="112"
            cy="112"
            r={radius}
            stroke={getStrokeColor()}
            strokeWidth="16"
            fill="none"
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            strokeLinecap="round"
            className="transition-all duration-500"
          />
        </svg>

        {/* Time display in center */}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <div className={`text-4xl font-black ${getColor()} tracking-tight`}>
            {timeFormatted.formatted}
          </div>
          <div className="text-gray-600 text-lg font-semibold mt-1">left</div>
        </div>
      </div>

      <div className="text-center">
        <div className="text-base font-semibold text-gray-800">Out of {formatMinutes(totalMinutes)} today</div>
      </div>
    </div>
  );
}
