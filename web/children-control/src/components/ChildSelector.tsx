// Child Selector Component for Login Screen

import type { ChildForAuth } from '../api/types';

interface ChildSelectorProps {
  children: ChildForAuth[];
  onSelect: (childId: string) => void;
  onSharedSession?: () => void;
}

export function ChildSelector({ children, onSelect, onSharedSession }: ChildSelectorProps) {
  return (
    <div className="w-full max-w-2xl mx-auto px-4">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {children.map((child) => (
          <button
            key={child.id}
            onClick={() => onSelect(child.id)}
            className="card transform transition-all hover:scale-105 active:scale-95 p-8 flex flex-col items-center gap-4 bg-gradient-to-br from-white to-purple-50 hover:shadow-2xl"
          >
            <div className="text-6xl">{child.emoji}</div>
            <div className="text-xl font-bold text-gray-800">{child.name}</div>
          </button>
        ))}

        {/* Shared Session Button */}
        {onSharedSession && children.length > 1 && (
          <button
            onClick={onSharedSession}
            className="card transform transition-all hover:scale-105 active:scale-95 p-8 flex flex-col items-center gap-4 bg-gradient-to-br from-purple-100 to-purple-200 hover:shadow-2xl border-2 border-purple-300"
          >
            <div className="text-6xl">👥</div>
            <div className="text-xl font-bold text-gray-800">Shared</div>
          </button>
        )}
      </div>
    </div>
  );
}
