<script setup>
import { RouterLink, RouterView } from 'vue-router'
</script>

<script>
export default {
  name: 'app',
  data() {
    return {
      locale: 'en',
    };
  },
  watch: {
    locale(val) {
      this.$i18n.locale = val;
    },
  },
  methods: {
    toggleLang() {
      this.locale = this.locale === 'en' ? 'de' : 'en';
    },
  },
};
</script>

<template>
  <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
    <div class="container-fluid">
      <a class="navbar-brand" href="/"><img src="/img/header-logo.png" alt="WireGuard Portal" /></a>
      <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarTop"
        aria-controls="navbarColor01" aria-expanded="false" aria-label="Toggle navigation">
        <span class="navbar-toggler-icon"></span>
      </button>

      <div class="collapse navbar-collapse" id="navbarTop">
        <ul class="navbar-nav me-auto">
          <li class="nav-item">
            <RouterLink class="nav-link" :to="{ name: 'home' }">{{ $t('menu.home') }}</RouterLink>
          </li>
          <li class="nav-item">
            <RouterLink class="nav-link" :to="{ name: 'interfaces' }">{{ $t('menu.interfaces') }}</RouterLink>
          </li>
          <li class="nav-item">
            <button @click="toggleLang">
              {{ $t('menu.lang') }}
            </button>

          </li>
        </ul>

        <div class="navbar-nav d-flex justify-content-end">
          <div class="nav-item dropdown">
            <a class="nav-link dropdown-toggle" data-bs-toggle="dropdown" href="#" role="button" aria-haspopup="true"
              aria-expanded="false">{{ $t('menu.firstname') }} {{ $t('menu.lastname') }}</a>
            <!-- <li class="nav-item">
            <RouterLink class="nav-link" :to="{ name: 'users' }">{{ $t('hello') }}</RouterLink>
          </li> -->
            <div class="dropdown-menu">
              <a class="dropdown-item" href="/admin"><i class="fas fa-cogs"></i> {{ $t('menu.admin') }}</a>
              <a class="dropdown-item" href="/admin/users"><i class="fas fa-users-cog"></i> {{ $t('menu.user') }}</a>
              <div class="dropdown-divider"></div>
              <a class="dropdown-item" href="/user/profile"><i class="fas fa-user"></i> {{ $t('menu.profile') }}</a>
              <div class="dropdown-divider"></div>
              <a class="dropdown-item" href="/auth/logout"><i class="fas fa-sign-out-alt"></i> {{ $t('menu.logout')
              }}</a>
            </div>
          </div>
          <div class="nav-item">
            <a href="/login" class="nav-link"><i class="fas fa-sign-in-alt fa-sm fa-fw me-2"></i>{{ $t('menu.login')
            }}</a>
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
      <p>Powered by <img height="20" src="@/assets/logo.svg" alt="Vue.JS" /></p>
    </div>
  </footer>
</template>
