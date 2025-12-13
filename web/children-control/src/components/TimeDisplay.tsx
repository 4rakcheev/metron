// Time Display Component with Circular Progress

interface TimeDisplayProps {
  remainingMinutes: number;
  totalMinutes: number;
}

export function TimeDisplay({ remainingMinutes, totalMinutes }: TimeDisplayProps) {
  const percentage = totalMinutes > 0 ? (remainingMinutes / totalMinutes) * 100 : 0;
  const radius = 80;
  const circumference = 2 * Math.PI * radius;
  const strokeDashoffset = circumference - (percentage / 100) * circumference;

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

  return (
    <div className="flex flex-col items-center gap-4 p-8">
      <div className="relative w-48 h-48">
        {/* Background circle */}
        <svg className="transform -rotate-90 w-48 h-48">
          <circle
            cx="96"
            cy="96"
            r={radius}
            stroke="#e5e7eb"
            strokeWidth="12"
            fill="none"
          />
          {/* Progress circle */}
          <circle
            cx="96"
            cy="96"
            r={radius}
            stroke={getStrokeColor()}
            strokeWidth="12"
            fill="none"
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            strokeLinecap="round"
            className="transition-all duration-500"
          />
        </svg>

        {/* Time display in center */}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <div className={`text-5xl font-bold ${getColor()}`}>
            {remainingMinutes}
          </div>
          <div className="text-gray-500 text-lg">minutes</div>
        </div>
      </div>

      <div className="text-center text-gray-600">
        <div className="text-sm">Out of {totalMinutes} minutes today</div>
      </div>
    </div>
  );
}
