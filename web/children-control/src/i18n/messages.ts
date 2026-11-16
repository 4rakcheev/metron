// Translations for the child app. Russian is the main language.

export type Lang = 'ru' | 'en';

export const DEFAULT_LANG: Lang = 'ru';
export const LANGS: Lang[] = ['ru', 'en'];

// Locale used for dates and clock times
export const LOCALES: Record<Lang, string> = {
  ru: 'ru-RU',
  en: 'en-GB',
};

const ru = {
  // Language switcher
  'lang.label': 'Язык',

  // Login page
  'login.titleSelect': 'Кто хочет поиграть? 🎮',
  'login.titleHi': 'Привет, {name}! 👋',
  'login.subtitleSelect': 'Выбери свое имя',
  'login.subtitlePin': 'Введи PIN, чтобы продолжить',
  'login.pinLabel': 'Введи PIN из 4 цифр',
  'login.back': 'Назад',
  'login.submit': 'Войти',
  'login.submitting': 'Входим...',
  'login.errSelectChild': 'Выбери свое имя',
  'login.errPinLength': 'PIN должен быть из 4 цифр',
  'login.errLoadChildren': 'Не получилось загрузить список детей',

  // Home page
  'home.loading': 'Загрузка...',
  'home.hi': 'Привет, {name}! 👋',
  'home.subtitle': 'Во что сегодня играем?',
  'home.allowedToday': '✅ Сегодня можно',
  'home.logout': 'Выйти',
  'home.activeTitle': '🎮 Идет игра',
  'home.chooseDevice': '📱 Выбери устройство',
  'home.starting': 'Запускаем...',
  'home.statsUsed': 'Потрачено',
  'home.statsRemaining': 'Осталось',
  'home.statsSessions': 'Сессий',

  'lockdown.title': 'Экранное время закрыто',
  'lockdown.text': 'Родители закрыли экранное время. Сейчас нельзя начать игру. Оставшееся время сохранится.',
  'lockdown.until': 'Закрыто до',

  'downtime.title': 'Время отдыха',
  'downtime.text': 'Сейчас время отдыха! Начать или продлить игру нельзя.',
  'downtime.ends': 'Отдых закончится в',

  'approval.title': 'Сначала уроки',
  'approval.text': 'Попроси родителей разрешить экранное время на сегодня. Страница обновится сама.',

  'noTime.title': 'На сегодня время закончилось',
  'noTime.text': 'Все экранное время на сегодня уже использовано. Приходи завтра!',

  'allLocked.title': 'Все устройства закрыты',
  'allLocked.text': 'Родители пока закрыли все устройства.',

  'extend.lockdown': '🔒 Экранное время закрыто. Добавить время нельзя.',
  'extend.downtime': '🌙 Сейчас время отдыха. Добавить время нельзя.',
  'extend.approval': '📚 Чтобы добавить время, попроси родителей разрешить экранное время на сегодня.',
  'extend.deviceLocked': '🔒 Это устройство закрыто. Добавить время нельзя.',

  'action.startFailed': 'Не получилось начать игру',
  'action.stopFailed': 'Не получилось остановить игру',
  'action.extendFailed': 'Не получилось добавить время',

  // Active session
  'active.playingOn': 'Играешь на',
  'active.remaining': 'осталось',
  'active.extend': '⏱️ Продлить',
  'active.stop': '🛑 Стоп',
  'active.howMuch': 'Сколько еще добавить?',
  'active.plusMin': '+{n} мин',
  'active.cancel': 'Отмена',

  // Device button
  'device.lockedHint': 'Родители закрыли это устройство',
  'device.lockedUntil': 'Закрыто до {time}',
  'device.locked': 'Закрыто',

  // Duration picker
  'duration.title': 'Сколько хочешь играть?',
  'duration.minutes': '{n} мин',
  'duration.hour': '1 час',
  'duration.available': '(доступно {n} мин)',
  'duration.noTime': 'Время закончилось',

  // Time display
  'time.left': 'осталось',
  'time.outOf': 'Из {total} на сегодня',
  'time.hours': '{h} ч',
  'time.minutes': '{m} мин',
  'time.hoursMinutes': '{h} ч {m} мин',

  // Errors
  'error.LOCKDOWN_ACTIVE': '🔒 Родители сейчас закрыли экранное время.',
  'error.DOWNTIME_ACTIVE': '🌙 Сейчас время отдыха. Экранное время недоступно.',
  'error.APPROVAL_REQUIRED': '📚 Попроси родителей разрешить экранное время на сегодня.',
  'error.INSUFFICIENT_TIME': '⏰ На сегодня не хватает времени.',
  'error.INVALID_CREDENTIALS': 'Неверный PIN',
  'error.sessionExpired': 'Нужно войти заново.',
  'error.network': 'Нет связи с сервером. Попробуй еще раз.',
  'error.generic': 'Что-то пошло не так. Попробуй еще раз.',
  'error.loadFailed': 'Не получилось загрузить данные',
};

export type MessageKey = keyof typeof ru;

const en: Record<MessageKey, string> = {
  'lang.label': 'Language',

  'login.titleSelect': 'Who wants to play? 🎮',
  'login.titleHi': 'Hi {name}! 👋',
  'login.subtitleSelect': 'Select your name below',
  'login.subtitlePin': 'Enter your PIN to continue',
  'login.pinLabel': 'Enter your 4-digit PIN',
  'login.back': 'Back',
  'login.submit': 'Login',
  'login.submitting': 'Logging in...',
  'login.errSelectChild': 'Please select your name',
  'login.errPinLength': 'PIN must be 4 digits',
  'login.errLoadChildren': 'Failed to load children list',

  'home.loading': 'Loading...',
  'home.hi': 'Hi, {name}! 👋',
  'home.subtitle': 'Ready to have fun?',
  'home.allowedToday': '✅ Allowed today',
  'home.logout': 'Logout',
  'home.activeTitle': '🎮 Active Session',
  'home.chooseDevice': '📱 Choose a device',
  'home.starting': 'Starting session...',
  'home.statsUsed': 'Used',
  'home.statsRemaining': 'Remaining',
  'home.statsSessions': 'Sessions',

  'lockdown.title': 'Screen time is locked',
  'lockdown.text': 'Your parents have locked screen time. No sessions can be started right now. Your remaining time is saved.',
  'lockdown.until': 'Locked until',

  'downtime.title': 'Downtime Period',
  'downtime.text': "It's rest time now! You cannot start or extend sessions during downtime.",
  'downtime.ends': 'Downtime ends at',

  'approval.title': 'First homework',
  'approval.text': 'Ask your parents to allow screen time for today. This page will update by itself.',

  'noTime.title': 'No time left today',
  'noTime.text': "You've used all your screen time for today. Come back tomorrow!",

  'allLocked.title': 'All devices are locked',
  'allLocked.text': 'Your parents have locked all devices for now.',

  'extend.lockdown': '🔒 Screen time is locked. You cannot add more time.',
  'extend.downtime': '🌙 It is rest time now. You cannot add more time.',
  'extend.approval': '📚 Ask your parents to allow screen time for today to add more time.',
  'extend.deviceLocked': '🔒 This device is locked. You cannot add more time.',

  'action.startFailed': 'Failed to start session',
  'action.stopFailed': 'Failed to stop session',
  'action.extendFailed': 'Failed to extend session',

  'active.playingOn': 'Currently playing on',
  'active.remaining': 'remaining',
  'active.extend': '⏱️ Extend',
  'active.stop': '🛑 Stop',
  'active.howMuch': 'How much more time?',
  'active.plusMin': '+{n} min',
  'active.cancel': 'Cancel',

  'device.lockedHint': 'This device is locked by your parents',
  'device.lockedUntil': 'Locked until {time}',
  'device.locked': 'Locked',

  'duration.title': 'How long do you want to play?',
  'duration.minutes': '{n} min',
  'duration.hour': '1 hour',
  'duration.available': '({n} min available)',
  'duration.noTime': 'No time left',

  'time.left': 'left',
  'time.outOf': 'Out of {total} today',
  'time.hours': '{h}h',
  'time.minutes': '{m}m',
  'time.hoursMinutes': '{h}h {m}m',

  'error.LOCKDOWN_ACTIVE': '🔒 Your parents have locked screen time right now.',
  'error.DOWNTIME_ACTIVE': '🌙 It is rest time now. Screen time is not available.',
  'error.APPROVAL_REQUIRED': '📚 Ask your parents to allow screen time for today.',
  'error.INSUFFICIENT_TIME': '⏰ Not enough time left today.',
  'error.INVALID_CREDENTIALS': 'Invalid PIN',
  'error.sessionExpired': 'Session expired. Please login again.',
  'error.network': 'Cannot reach the server. Please try again.',
  'error.generic': 'Something went wrong. Please try again.',
  'error.loadFailed': 'Failed to load data',
};

export const MESSAGES: Record<Lang, Record<MessageKey, string>> = { ru, en };
