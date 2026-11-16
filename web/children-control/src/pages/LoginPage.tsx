// Login Page with Child Selection and PIN Entry

import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { api } from '../api/client';
import { ChildSelector } from '../components/ChildSelector';
import { LanguageSwitcher } from '../components/LanguageSwitcher';
import { useI18n } from '../i18n/LanguageContext';
import type { ChildForAuth } from '../api/types';

export function LoginPage() {
  const navigate = useNavigate();
  const { isAuthenticated, login } = useApp();
  const { t } = useI18n();
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
        setError(t('login.errLoadChildren'));
        console.error(err);
      }
    }
    loadChildren();
  }, [t]);

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
      setError(t('login.errSelectChild'));
      return;
    }

    if (pin.length !== 4) {
      setError(t('login.errPinLength'));
      return;
    }

    try {
      setLoading(true);
      setError('');
      await login(selectedChildId, pin);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('error.INVALID_CREDENTIALS'));
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
    <div className="min-h-screen flex flex-col items-center justify-center p-4 relative">
      <div className="absolute top-4 right-4">
        <LanguageSwitcher />
      </div>
      <div className="w-full max-w-2xl">
        {/* Title */}
        <div className="text-center mb-8">
          <h1 className="text-5xl font-bold text-transparent bg-clip-text bg-gradient-to-r from-purple-600 to-pink-600 mb-2">
            {selectedChildId ? t('login.titleHi', { name: selectedChild?.name ?? '' }) : t('login.titleSelect')}
          </h1>
          <p className="text-gray-600 text-lg">
            {selectedChildId ? t('login.subtitlePin') : t('login.subtitleSelect')}
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
                  {t('login.pinLabel')}
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
                  {t('login.back')}
                </button>
                <button
                  type="submit"
                  disabled={loading || pin.length !== 4}
                  className="flex-1 btn-primary"
                >
                  {loading ? t('login.submitting') : t('login.submit')}
                </button>
              </div>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
