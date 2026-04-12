<script setup>
import { RouterLink, RouterView } from 'vue-router';
import {computed, getCurrentInstance, nextTick, onMounted, ref} from "vue";
import { authStore } from "./stores/auth";
import { securityStore } from "./stores/security";
import { settingsStore } from "@/stores/settings";
import { Notifications } from "@kyvg/vue3-notification";

const appGlobal = getCurrentInstance().appContext.config.globalProperties
const auth = authStore()
const sec = securityStore()
const settings = settingsStore()

const currentTheme = ref("auto")

onMounted(async () => {
  console.log("Starting WireGuard Portal frontend...");

  // restore theme from localStorage
  switchTheme(getTheme());

  await sec.LoadSecurityProperties();
  await auth.LoadProviders();

  let wasLoggedIn = auth.IsAuthenticated;
  try {
    await auth.LoadSession();
    await settings.LoadSettings(); // only logs errors, does not throw

    console.log("WireGuard Portal session is valid");
  } catch (e) {
    if (wasLoggedIn) {
      console.log("WireGuard Portal invalid - logging out");
      await auth.Logout();
    }
  }

  if (!wasLoggedIn && window.location.href != '/app/#/login') {
    window.location.href = '/app/#/login';
    return
  }

  console.log("WireGuard Portal ready!");
})

const switchLanguage = function (lang) {
  if (appGlobal.$i18n.locale !== lang) {
    localStorage.setItem('wgLang', lang);
    appGlobal.$i18n.locale = lang;
  }
}

const getTheme = function () {
  return localStorage.getItem('wgTheme') || 'auto';
}

const switchTheme = function (theme) {
  let bsTheme = theme;
  if (theme === 'auto') {
    bsTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  currentTheme.value = theme;

  if (document.documentElement.getAttribute('data-bs-theme') !== bsTheme) {
    console.log("Switching theme to " + theme + " (" + bsTheme + ")");
    localStorage.setItem('wgTheme', theme);
    document.documentElement.setAttribute('data-bs-theme', bsTheme);
  }
}

const languageFlag = computed(() => {
  // `this` points to the component instance
  let lang = appGlobal.$i18n.locale.toLowerCase();
  if (!appGlobal.$i18n.availableLocales.includes(lang)) {
    lang = appGlobal.$i18n.fallbackLocale;
  }
  const langMap = {
    en: "us",
    pt: "pt",
    uk: "ua",
    zh: "cn",
    ko: "kr",
    es: "es",

  };
  return "fi-" + (langMap[lang] || lang);
})

const companyName = ref(WGPORTAL_SITE_COMPANY_NAME);
const wgVersion = ref(WGPORTAL_VERSION);
const currentYear = ref(new Date().getFullYear())
const webBasePath = ref(WGPORTAL_BASE_PATH);

const userDisplayName = computed(() => {
  let displayName = "Unknown";
  if (auth.IsAuthenticated) {
    if (auth.User.Firstname === "" && auth.User.Lastname === "") {
      displayName = auth.User.Identifier;
    } else if (auth.User.Firstname === "" && auth.User.Lastname !== "") {
      displayName = auth.User.Lastname;
    } else if (auth.User.Firstname !== "" && auth.User.Lastname === "") {
      displayName = auth.User.Firstname;
    } else if (auth.User.Firstname !== "" && auth.User.Lastname !== "") {
      displayName = auth.User.Firstname + " " + auth.User.Lastname;
    }
  }

  // pad string to 20 characters so that the menu is always the same size on desktop
  if (displayName.length < 20 && window.innerWidth > 992) {
    displayName = displayName.padStart(20, "\u00A0");
  }
  return displayName;
})
</script>

<template>
  <notifications :duration="3000" :ignore-duplicates="true" position="top right" />

  <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
    <div class="container-fluid">
      <RouterLink class="navbar-brand" :to="{ name: 'home' }"><img :alt="companyName" :src="webBasePath + '/img/header-logo.png'" /></RouterLink>
      <button aria-controls="navbarColor01" aria-expanded="false" aria-label="Toggle navigation" class="navbar-toggler"
        data-bs-target="#navbarTop" data-bs-toggle="collapse" type="button">
        <span class="navbar-toggler-icon"></span>
      </button>

      <div id="navbarTop" class="collapse navbar-collapse">
        <ul class="navbar-nav me-auto">
          <li class="nav-item">
            <RouterLink :to="{ name: 'home' }" class="nav-link">{{ $t('menu.home') }}</RouterLink>
          </li>
          <li v-if="auth.IsAuthenticated && auth.IsAdmin" class="nav-item">
            <RouterLink :to="{ name: 'interfaces' }" class="nav-link">{{ $t('menu.interfaces') }}</RouterLink>
          </li>
          <li v-if="auth.IsAuthenticated && auth.IsAdmin" class="nav-item">
            <RouterLink :to="{ name: 'users' }" class="nav-link">{{ $t('menu.users') }}</RouterLink>
          </li>
          <li class="nav-item">
            <RouterLink :to="{ name: 'key-generator' }" class="nav-link">{{ $t('menu.keygen') }}</RouterLink>
          </li>
          <li class="nav-item">
            <RouterLink :to="{ name: 'ip-calculator' }" class="nav-link">{{ $t('menu.calculator') }}</RouterLink>
          </li>
        </ul>

        <div class="navbar-nav d-flex justify-content-end">
          <div v-if="auth.IsAuthenticated" class="nav-item dropdown">
            <a aria-expanded="false" aria-haspopup="true" class="nav-link dropdown-toggle" data-bs-toggle="dropdown"
              href="#" role="button">{{ userDisplayName }}</a>
            <div class="dropdown-menu">
              <RouterLink :to="{ name: 'profile' }" class="dropdown-item"><i class="fas fa-user"></i> {{ $t('menu.profile') }}</RouterLink>
              <RouterLink :to="{ name: 'settings' }" class="dropdown-item" v-if="auth.IsAdmin || !settings.Setting('ApiAdminOnly') || settings.Setting('WebAuthnEnabled')"><i class="fas fa-gears"></i> {{ $t('menu.settings') }}</RouterLink>
              <RouterLink :to="{ name: 'audit' }" class="dropdown-item" v-if="auth.IsAdmin"><i class="fas fa-file-shield"></i> {{ $t('menu.audit') }}</RouterLink>
              <div class="dropdown-divider"></div>
              <a class="dropdown-item" href="#" @click.prevent="auth.Logout"><i class="fas fa-sign-out-alt"></i> {{ $t('menu.logout') }}</a>
            </div>
          </div>
          <div v-if="!auth.IsAuthenticated" class="nav-item">
            <RouterLink :to="{ name: 'login' }" class="nav-link"><i class="fas fa-sign-in-alt fa-sm fa-fw me-2"></i>{{ $t('menu.login') }}</RouterLink>
          </div>
          <div class="nav-item dropdown" :key="currentTheme">
            <a class="nav-link dropdown-toggle d-flex align-items-center" href="#" id="theme-menu" aria-expanded="false" data-bs-toggle="dropdown" data-bs-display="static" aria-label="Toggle theme">
              <i class="fa-solid fa-circle-half-stroke"></i>
              <span class="d-lg-none ms-2">Toggle theme</span>
            </a>
            <ul class="dropdown-menu dropdown-menu-end">
              <li>
                <button type="button" class="dropdown-item d-flex align-items-center" @click.prevent="switchTheme('auto')" aria-pressed="false">
                  <i class="fa-solid fa-circle-half-stroke"></i><span class="ms-2">System</span><i class="fa-solid fa-check ms-5" :class="{invisible:currentTheme!=='auto'}"></i>
                </button>
              </li>
              <li>
                <button type="button" class="dropdown-item d-flex align-items-center" @click.prevent="switchTheme('light')" aria-pressed="false">
                  <i class="fa-solid fa-sun"></i><span class="ms-2">Light</span><i class="fa-solid fa-check ms-5" :class="{invisible:currentTheme!=='light'}"></i>
                </button>
              </li>
              <li>
                <button type="button" class="dropdown-item d-flex align-items-center" @click.prevent="switchTheme('dark')" aria-pressed="true">
                  <i class="fa-solid fa-moon"></i><span class="ms-2">Dark</span><i class="fa-solid fa-check ms-5" :class="{invisible:currentTheme!=='dark'}"></i>
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </nav>

  <div class="container mt-5 flex-shrink-0">
    <RouterView />
  </div>

  <footer class="page-footer mt-auto">
    <div class="container mt-5">
      <div class="row align-items-center">
        <div class="col-6">Copyright © {{ companyName }} {{ currentYear }} <span v-if="auth.IsAuthenticated"> - version {{ wgVersion }}</span></div>
        <div class="col-6 text-end">
          <div :aria-label="$t('menu.lang')" class="btn-group" role="group">
            <div class="btn-group" role="group">
              <button aria-expanded="false" aria-haspopup="true" class="btn flag-button pe-0"
                data-bs-toggle="dropdown" type="button"><span :class="languageFlag" class="fi"></span></button>
              <div aria-labelledby="btnGroupDrop3" class="dropdown-menu" style="">
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('de')"><span class="fi fi-de"></span> Deutsch</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('en')"><span class="fi fi-us"></span> English</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('fr')"><span class="fi fi-fr"></span> Français</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('ko')"><span class="fi fi-kr"></span> 한국어</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('pt')"><span class="fi fi-pt"></span> Português</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('ru')"><span class="fi fi-ru"></span> Русский</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('uk')"><span class="fi fi-ua"></span> Українська</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('vi')"><span class="fi fi-vi"></span> Tiếng Việt</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('zh')"><span class="fi fi-cn"></span> 中文</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('es')"><span class="fi fi-es"></span> Español</a>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </footer>
</template>

<style>
.flag-button:active,.flag-button:hover,.flag-button:focus,.flag-button:checked,.flag-button:disabled,.flag-button:not(:disabled) {
  border: 1px solid transparent!important;
}
[data-bs-theme=dark] .form-select {
  color: #0c0c0c!important;
  background-color: #c1c1c1!important;
  --bs-form-select-bg-img: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3e%3cpath fill='none' stroke='%23343a40' stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='m2 5 6 6 6-6'/%3e%3c/svg%3e")!important;
}
[data-bs-theme=dark] .form-control {
  color: #0c0c0c!important;
  background-color: #c1c1c1!important;
}
[data-bs-theme=dark] .form-control:focus {
  color: #0c0c0c!important;
  background-color: #c1c1c1!important;
}
[data-bs-theme=dark] .badge.bg-light {
  --bs-bg-opacity: 1;
  background-color: rgba(var(--bs-dark-rgb), var(--bs-bg-opacity)) !important;
  color: var(--bs-badge-color)!important;
}
[data-bs-theme=dark] span.input-group-text {
  --bs-bg-opacity: 1;
  background-color: rgba(var(--bs-dark-rgb), var(--bs-bg-opacity)) !important;
  color: var(--bs-badge-color)!important;
}

[data-bs-theme=dark] .navbar-dark, .navbar {
  background-color: #000 !important;
}
</style>
