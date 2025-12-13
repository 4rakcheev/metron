// Login Page with Child Selection and PIN Entry

import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { api } from '../api/client';
import { ChildSelector } from '../components/ChildSelector';
import type { ChildForAuth } from '../api/types';

export function LoginPage() {
  const navigate = useNavigate();
  const { isAuthenticated, login } = useApp();
  const [children, setChildren] = useState<ChildForAuth[]>([]);
  const [selectedChildId, setSelectedChildId] = useState<string | null>(null);
  const [pin, setPin] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/');
    }
  }, [isAuthenticated, navigate]);

  // Load children list
  useEffect(() => {
    async function loadChildren() {
      try {
        const childrenList = await api.listChildren();
        setChildren(childrenList);
      } catch (err) {
        setError('Failed to load children list');
        console.error(err);
      }
    }
    loadChildren();
  }, []);

  // Handle child selection
  const handleChildSelect = (childId: string) => {
    setSelectedChildId(childId);
    setError('');
    setPin('');
  };

  // Handle PIN input
  const handlePinChange = (value: string) => {
    // Only allow 4 digits
    if (/^\d{0,4}$/.test(value)) {
      setPin(value);
      setError('');
    }
  };

  // Handle login submission
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedChildId) {
      setError('Please select a child');
      return;
    }

    if (pin.length !== 4) {
      setError('PIN must be 4 digits');
      return;
    }

    try {
      setLoading(true);
      setError('');
      await login(selectedChildId, pin);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid PIN');
      setPin('');
    } finally {
      setLoading(false);
    }
  };

  // Handle back to child selection
  const handleBack = () => {
    setSelectedChildId(null);
    setPin('');
    setError('');
  };

  const selectedChild = children.find(c => c.id === selectedChildId);

  return (
    <div className="min-h-screen flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-2xl">
        {/* Title */}
        <div className="text-center mb-8">
          <h1 className="text-5xl font-bold text-transparent bg-clip-text bg-gradient-to-r from-purple-600 to-pink-600 mb-2">
            {selectedChildId ? `Hi ${selectedChild?.name}! 👋` : 'Who wants to play? 🎮'}
          </h1>
          <p className="text-gray-600 text-lg">
            {selectedChildId ? 'Enter your PIN to continue' : 'Select your name below'}
          </p>
        </div>

        {/* Child Selection */}
        {!selectedChildId && (
          <ChildSelector children={children} onSelect={handleChildSelect} />
        )}

        {/* PIN Entry */}
        {selectedChildId && (
          <div className="card max-w-md mx-auto">
            <form onSubmit={handleLogin} className="flex flex-col gap-6">
              {/* PIN Input */}
              <div>
                <label className="block text-gray-700 font-semibold mb-2 text-center text-lg">
                  Enter your 4-digit PIN
                </label>
                <input
                  type="password"
                  inputMode="numeric"
                  pattern="\d{4}"
                  maxLength={4}
                  value={pin}
                  onChange={(e) => handlePinChange(e.target.value)}
                  className="w-full text-center text-3xl font-bold tracking-widest px-4 py-4 border-2 border-gray-300 rounded-2xl focus:border-purple-500 focus:outline-none"
                  placeholder="••••"
                  autoFocus
                  disabled={loading}
                />
              </div>

              {/* Error Message */}
              {error && (
                <div className="bg-red-50 border-2 border-red-200 rounded-2xl p-4 text-red-600 text-center font-semibold">
                  {error}
                </div>
              )}

              {/* Buttons */}
              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={handleBack}
                  disabled={loading}
                  className="flex-1 bg-gray-200 text-gray-700 font-bold py-3 px-6 rounded-2xl hover:bg-gray-300 transition disabled:opacity-50"
                >
                  Back
                </button>
                <button
                  type="submit"
                  disabled={loading || pin.length !== 4}
                  className="flex-1 btn-primary"
                >
                  {loading ? 'Logging in...' : 'Login'}
                </button>
              </div>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
