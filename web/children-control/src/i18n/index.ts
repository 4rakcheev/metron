// Small i18n helper without extra libraries.
// The current language lives in this module so non-React code (API client,
// time formatting) can use it too. React components re-render through
// LanguageProvider when the language changes.

import { DEFAULT_LANG, LANGS, LOCALES, MESSAGES, type Lang, type MessageKey } from './messages';

export type { Lang, MessageKey } from './messages';
export { LANGS } from './messages';

const LANG_KEY = 'metron_child_lang';

export type TranslateParams = Record<string, string | number>;

function isLang(value: unknown): value is Lang {
  return typeof value === 'string' && (LANGS as string[]).includes(value);
}

function loadSavedLang(): Lang {
  try {
    const saved = localStorage.getItem(LANG_KEY);
    if (isLang(saved)) return saved;
  } catch {
    // Storage can be blocked (private mode); use the default
  }
  return DEFAULT_LANG;
}

let currentLang: Lang = loadSavedLang();

export function getLang(): Lang {
  return currentLang;
}

export function getLocale(lang: Lang = currentLang): string {
  return LOCALES[lang];
}

// Changes the language for non-React code and remembers it on this device
export function setCurrentLang(lang: Lang): void {
  currentLang = lang;
  try {
    localStorage.setItem(LANG_KEY, lang);
  } catch {
    // Ignore: the language just will not be remembered
  }
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lang;
  }
}

export function hasMessage(key: string): key is MessageKey {
  return key in MESSAGES[DEFAULT_LANG];
}

// Translates a key and fills {placeholders} from params
export function translate(key: MessageKey, params?: TranslateParams, lang: Lang = currentLang): string {
  const template = MESSAGES[lang][key] ?? MESSAGES[DEFAULT_LANG][key] ?? key;
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in params ? String(params[name]) : match,
  );
}
