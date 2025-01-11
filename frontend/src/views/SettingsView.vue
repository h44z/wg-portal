<script setup>
import PeerViewModal from "../components/PeerViewModal.vue";

import { onMounted, ref } from "vue";
import { profileStore } from "@/stores/profile";
import PeerEditModal from "@/components/PeerEditModal.vue";
import { settingsStore } from "@/stores/settings";
import { humanFileSize } from "@/helpers/utils";
import {RouterLink} from "vue-router";
import {authStore} from "../stores/auth";

const profile = profileStore()
const settings = settingsStore()
const auth = authStore()

onMounted(async () => {
  await profile.LoadUser()
})

</script>

<template>
  <div class="page-header">
    <h1>{{ $t('settings.headline') }}</h1>
  </div>

  <p class="lead">{{ $t('settings.abstract') }}</p>

  <div v-if="auth.IsAdmin || !settings.Setting('ApiAdminOnly')">
    <div class="bg-light p-5" v-if="profile.user.ApiToken">
      <h2 class="display-7">{{ $t('settings.api.headline') }}</h2>
      <p class="lead">{{ $t('settings.api.abstract') }}</p>
      <hr class="my-4">
      <p>{{ $t('settings.api.active-description') }}</p>
      <div class="row">
        <div class="col-6">
          <div class="form-group">
            <label class="form-label mt-4">{{ $t('settings.api.user-label') }}</label>
            <input v-model="profile.user.Identifier" class="form-control" :placeholder="$t('settings.api.user-placeholder')" type="text" readonly>
          </div>
        </div>
        <div class="col-6">
          <div class="form-group">
            <label class="form-label mt-4">{{ $t('settings.api.token-label') }}</label>
            <input v-model="profile.user.ApiToken" class="form-control" :placeholder="$t('settings.api.token-placeholder')" type="text" readonly>
          </div>
        </div>
      </div>
      <div class="row">
        <div class="col-12">
          <div class="form-group">
            <p class="form-label mt-4">{{ $t('settings.api.token-created-label') }} {{profile.user.ApiTokenCreated}}</p>
          </div>
        </div>
      </div>
      <div class="row mt-5">
        <div class="col-6">
          <button class="input-group-text btn btn-primary" :title="$t('settings.api.button-disable-title')" @click.prevent="profile.disableApi()" :disabled="profile.isFetching">
            <i class="fa-solid fa-minus-circle"></i> {{ $t('settings.api.button-disable-text') }}
          </button>
        </div>
        <div class="col-6">
          <a href="/api/v1/doc.html" target="_blank" :alt="$t('settings.api.api-link')">{{ $t('settings.api.api-link') }}</a>
        </div>
      </div>
    </div>
    <div class="bg-light p-5" v-else>
      <h2 class="display-7">{{ $t('settings.api.headline') }}</h2>
      <p class="lead">{{ $t('settings.api.abstract') }}</p>
      <hr class="my-4">
      <p>{{ $t('settings.api.inactive-description') }}</p>
      <button class="input-group-text btn btn-primary" :title="$t('settings.api.button-enable-title')" @click.prevent="profile.enableApi()" :disabled="profile.isFetching">
        <i class="fa-solid fa-plus-circle"></i> {{ $t('settings.api.button-enable-text') }}
      </button>
    </div>
  </div>
</template>
