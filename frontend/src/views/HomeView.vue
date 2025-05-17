<script setup>
import { authStore } from "@/stores/auth";
import { RouterLink } from "vue-router";

const auth = authStore()
</script>

<template>
  <div class="page-header">
    <h1>{{ $t('home.headline') }}</h1>
  </div>

  <p class="lead">{{ $t('home.abstract') }}</p>


  <div class="bg-light p-5" v-if="auth.IsAuthenticated">
    <h2 class="display-5">{{ $t('home.profiles.headline') }}</h2>
    <p class="lead">{{ $t('home.profiles.abstract') }}</p>
    <hr class="my-4">
    <p>{{ $t('home.profiles.content') }}</p>
    <p class="lead">
      <RouterLink :to="{ name: 'profile' }" class="btn btn-primary btn-lg">{{ $t('home.profiles.button') }}</RouterLink>
    </p>
  </div>

  <div class="bg-light p-5 mt-4" v-if="auth.IsAuthenticated && auth.IsAdmin">
    <h2 class="display-5">{{ $t('home.admin.headline') }}</h2>
    <p class="lead">{{ $t('home.admin.abstract') }}</p>
    <hr class="my-4">
    <p>{{ $t('home.admin.content') }}</p>
    <p class="lead">
      <RouterLink :to="{ name: 'interfaces' }" class="btn btn-primary btn-lg me-2">{{ $t('home.admin.button-admin') }}
      </RouterLink>
      <RouterLink :to="{ name: 'users' }" class="btn btn-primary btn-lg">{{ $t('home.admin.button-user') }}</RouterLink>
    </p>
  </div>

  <h3 class="mt-5">{{ $t('home.info-headline') }}</h3>
  <div class="row">
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.installation.box-header') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.installation.headline') }}</h4>
          <p class="card-text">{{ $t('home.installation.content') }}</p>
          <a href="https://www.wireguard.com/install/" title="WireGuard Installation" target="_blank"
            rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">{{ $t('home.installation.button') }}</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.about-wg.box-header') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.about-wg.headline') }}</h4>
          <p class="card-text">{{ $t('home.about-wg.content') }}</p>
          <a href="https://www.wireguard.com/" title="WireGuard" target="_blank" rel="noopener noreferrer"
            class="mt-auto btn btn-primary btn-sm">{{ $t('home.about-wg.button') }}</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">{{ $t('home.about-portal.box-header') }}</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">{{ $t('home.about-portal.headline') }}</h4>
          <p class="card-text">{{ $t('home.about-portal.content') }}</p>
          <a href="https://wgportal.org/" title="WireGuard Portal" target="_blank"
            rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">{{ $t('home.about-portal.button') }}</a>
        </div>
      </div>
    </div>
  </div>
</template>
