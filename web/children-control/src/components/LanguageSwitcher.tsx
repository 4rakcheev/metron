// Small RU / EN switch

import { LANGS } from '../i18n';
import { useI18n } from '../i18n/LanguageContext';

export function LanguageSwitcher() {
  const { lang, setLang, t } = useI18n();

  return (
    <div
      role="group"
      aria-label={t('lang.label')}
      className="inline-flex rounded-xl bg-gray-200 p-1 text-xs font-bold"
    >
      {LANGS.map((code) => (
        <button
          key={code}
          type="button"
          onClick={() => setLang(code)}
          aria-pressed={lang === code}
          className={`px-2 py-1 rounded-lg uppercase transition ${
            lang === code ? 'bg-white text-purple-600 shadow' : 'text-gray-600 hover:text-gray-800'
          }`}
        >
          {code}
        </button>
      ))}
    </div>
  );
}
