<script setup>
import { RouterLink, RouterView } from 'vue-router';
import {computed, getCurrentInstance, onMounted} from "vue";
import {authStore} from "./stores/auth";
import {securityStore} from "./stores/security";

const appGlobal = getCurrentInstance().appContext.config.globalProperties
const auth = authStore()
const sec = securityStore()

onMounted(async () => {
  console.log("Starting WireGuard Portal frontend...");

  await sec.LoadSecurityProperties();
  await auth.LoadProviders();

  let wasLoggedIn = auth.IsAuthenticated;
  try {
    await auth.LoadSession();
  } catch (e) {
    if (wasLoggedIn) {
      await auth.Logout();
    }
  }

  console.log("WireGuard Portal ready!");
})

const switchLanguage = function (lang) {
  if (appGlobal.$i18n.locale !== lang) {
    localStorage.setItem('wgLang', lang);
    appGlobal.$i18n.locale = lang;
  }
}

const languageFlag = computed(() => {
  // `this` points to the component instance
  let lang = appGlobal.$i18n.locale.toLowerCase();
  if (lang === "en") {
    lang = "us";
  }
  return "fi-" + lang;
})
</script>

<template>
  <notifications :duration="3000" :ignore-duplicates="true" position="top right" />

  <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
    <div class="container-fluid">
      <a class="navbar-brand" href="/"><img alt="WireGuard Portal" src="/img/header-logo.png" /></a>
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
        </ul>

        <div class="navbar-nav d-flex justify-content-end">
          <div v-if="auth.IsAuthenticated" class="nav-item dropdown">
            <a aria-expanded="false" aria-haspopup="true" class="nav-link dropdown-toggle" data-bs-toggle="dropdown" href="#"
              role="button">{{ auth.User.Firstname }} {{ auth.User.Lastname }}</a>
            <div class="dropdown-menu">
              <a class="dropdown-item" href="/user/profile">
                <i class="fas fa-user"></i> {{ $t('menu.profile') }}
              </a>
              <div class="dropdown-divider"></div>
              <a class="dropdown-item" href="#" @click.prevent="auth.Logout">
                <i class="fas fa-sign-out-alt"></i> {{ $t('menu.logout') }}
              </a>
            </div>
          </div>
          <div v-if="!auth.IsAuthenticated" class="nav-item">
            <RouterLink :to="{ name: 'login' }" class="nav-link">
              <i class="fas fa-sign-in-alt fa-sm fa-fw me-2"></i>{{ $t('menu.login') }}
            </RouterLink>
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
        <div class="col-6">Powered by <img alt="Vue.JS" height="20" src="@/assets/logo.svg" /></div>
        <div class="col-6 text-end">
          <div aria-label="{{ $t('menu.lang') }}" class="btn-group" role="group">
            <div class="btn-group" role="group">
              <button aria-expanded="false" aria-haspopup="true" class="btn btn btn-secondary pe-0" data-bs-toggle="dropdown" type="button"><span :class="languageFlag" class="fi"></span></button>
              <div aria-labelledby="btnGroupDrop3" class="dropdown-menu" style="">
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('en')"><span class="fi fi-us"></span> English</a>
                <a class="dropdown-item" href="#" @click.prevent="switchLanguage('de')"><span class="fi fi-de"></span> Deutsch</a>
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
.vue-notification-group {
  margin-top:5px;
}
</style>