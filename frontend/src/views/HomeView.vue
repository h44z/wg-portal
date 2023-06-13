<script setup>
  import {authStore} from "../stores/auth";

  const auth = authStore()
</script>

<template>
  <div class="page-header">
    <h1>{{ $t('home.h1') }}</h1>
  </div>

  <p class="lead">{{ $t('home.abstract') }}</p>


  <div class="bg-light p-5" v-if="auth.IsAuthenticated">
    <h2 class="display-5">{{ $t('home.profiles.h1') }}</h2>
    <p class="lead">{{ $t('home.profiles.abstract') }}</p>
    <hr class="my-4">
    <p>{{ $t('home.profiles.instruct') }}</p>
    <p class="lead">
      <a href="/user/profile" class="btn btn-primary btn-lg" title="User-Profile">{{ $t('home.profiles.btn') }}</a>
    </p>
  </div>

  <div class="bg-light p-5 mt-4" v-if="auth.IsAuthenticated && auth.IsAdmin">
    <h2 class="display-5">{{ $t('home.admin.h1') }}</h2>
    <p class="lead">{{ $t('home.admin.abstract') }}</p>
    <hr class="my-4">
    <p>{{ $t('home.admin.instruct') }}</p>
    <p class="lead">
      <a href="/admin/" class="btn btn-primary btn-lg me-2" title="WireGuard Administration">{{ $t('home.admin.btn-1') }}</a>
      <a href="/admin/users/" class="btn btn-primary btn-lg" title="User Administration">{{ $t('home.admin.btn-2') }}</a>
    </p>
  </div>

  <h3 class="mt-5">{{ $t('home.info') }}</h3>
  <div class="row">
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.installation.h1') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.installation.h2') }}</h4>
          <p class="card-text">{{ $t('home.installation.instruct') }}</p>
          <a href="https://www.wireguard.com/install/" title="WireGuard Installation" target="_blank"
            rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">{{ $t('home.installation.btn') }}</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.about-wg.h1') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.about-wg.h2') }}</h4>
          <p class="card-text">{{ $t('home.about-wg.instruct') }}</p>
          <a href="https://www.wireguard.com/" title="WireGuard" target="_blank" rel="noopener noreferrer"
            class="mt-auto btn btn-primary btn-sm">{{ $t('home.about-wg.btn') }}</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.about-portal.h1') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.about-portal.h2') }}</h4>
          <p class="card-text">{{ $t('home.about-portal.instruct') }}</p>
          <a href="https://github.com/h44z/wg-portal/" title="WireGuard Portal" target="_blank"
            rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">{{ $t('home.about-portal.btn') }}</a>
        </div>
      </div>
    </div>
  </div>
</template>
