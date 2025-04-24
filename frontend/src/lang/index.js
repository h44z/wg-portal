// src/lang/index.js
import de from './translations/de.json';
import en from './translations/en.json';
import fr from './translations/fr.json';
import ko from './translations/ko.json';
import pt from './translations/pt.json';
import ru from './translations/ru.json';
import uk from './translations/uk.json';
import vi from './translations/vi.json';
import zh from './translations/zh.json';

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
    "en": en,
    "fr": fr,
    "ko": ko,
    "pt": pt,
    "ru": ru,
    "uk": uk,
    "vi": vi,
    "zh": zh,
  }
});

export default i18n
