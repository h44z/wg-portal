// src/lang/index.js
import de from './translations/de.json';
import ru from './translations/ru.json';
import en from './translations/en.json';
import {createI18n} from "vue-i18n";

// Create i18n instance with options
const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  allowComposition: true,
  locale: (
    localStorage.getItem('wgLang')
    || (window && window.navigator && (window.navigator.userLanguage || window.navigator.language).split('-')[0])
    || 'en'
  ), // set locale
  fallbackLocale: "en", // set fallback locale
  messages: {
    "de": de,
    "ru": ru,
    "en": en
  }
});

export default i18n
