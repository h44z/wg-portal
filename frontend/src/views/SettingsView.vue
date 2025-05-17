<script setup>
import {onMounted, ref} from "vue";
import { profileStore } from "@/stores/profile";
import { settingsStore } from "@/stores/settings";
import { authStore } from "../stores/auth";

const profile = profileStore()
const settings = settingsStore()
const auth = authStore()

onMounted(async () => {
  await profile.LoadUser()
  await auth.LoadWebAuthnCredentials()
})

const selectedCredential = ref({})

function enableRename(credential) {
  credential.renameMode = true;
  credential.tempName = credential.Name; // Store the original name
}

function cancelRename(credential) {
  credential.renameMode = false;
  credential.tempName = null; // Discard changes
}

async function saveRename(credential) {
  try {
    await auth.RenameWebAuthnCredential({ ...credential, Name: credential.tempName });
    credential.Name = credential.tempName; // Update the name
    credential.renameMode = false;
  } catch (error) {
    console.error("Failed to rename credential:", error);
  }
}
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

  <div class="bg-light p-5 mt-5" v-if="settings.Setting('WebAuthnEnabled')">
    <h2 class="display-7">{{ $t('settings.webauthn.headline') }}</h2>
    <p class="lead">{{ $t('settings.webauthn.abstract') }}</p>
    <hr class="my-4">
    <p v-if="auth.IsWebAuthnEnabled">{{ $t('settings.webauthn.active-description') }}</p>
    <p v-else>{{ $t('settings.webauthn.inactive-description') }}</p>

    <div class="row">
      <div class="col-6">
        <button class="input-group-text btn btn-primary" :title="$t('settings.webauthn.button-register-text')" @click.prevent="auth.RegisterWebAuthn" :disabled="auth.isFetching">
          <i class="fa-solid fa-plus-circle"></i> {{ $t('settings.webauthn.button-register-title') }}
        </button>
      </div>
    </div>

    <div v-if="auth.WebAuthnCredentials.length > 0" class="mt-4">
      <h3>{{ $t('settings.webauthn.credentials-list') }}</h3>
      <table class="table table-striped">
        <thead>
        <tr>
          <th style="width: 50%">{{ $t('settings.webauthn.table.name') }}</th>
          <th style="width: 20%">{{ $t('settings.webauthn.table.created') }}</th>
          <th style="width: 30%">{{ $t('settings.webauthn.table.actions') }}</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="credential in auth.webAuthnCredentials" :key="credential.ID">
          <td class="align-middle">
            <div v-if="credential.renameMode">
              <input v-model="credential.tempName" class="form-control" type="text" />
            </div>
            <div v-else>
              {{ credential.Name }}
            </div>
          </td>
          <td class="align-middle">
            {{ credential.CreatedAt }}
          </td>
          <td class="align-middle text-center">
            <div v-if="credential.renameMode">
              <button class="btn btn-success me-1" :title="$t('settings.webauthn.button-save-text')" @click.prevent="saveRename(credential)" :disabled="auth.isFetching">
                {{ $t('settings.webauthn.button-save-title') }}
              </button>
              <button class="btn btn-secondary" :title="$t('settings.webauthn.button-cancel-text')" @click.prevent="cancelRename(credential)">
                {{ $t('settings.webauthn.button-cancel-title') }}
              </button>
            </div>
            <div v-else>
              <button class="btn btn-secondary me-1" :title="$t('settings.webauthn.button-rename-text')" @click.prevent="enableRename(credential)">
                {{ $t('settings.webauthn.button-rename-title') }}
              </button>
              <button class="btn btn-danger" :title="$t('settings.webauthn.button-delete-text')" data-bs-toggle="modal" data-bs-target="#webAuthnDeleteModal" :disabled="auth.isFetching" @click="selectedCredential=credential">
                {{ $t('settings.webauthn.button-delete-title') }}
              </button>
            </div>
          </td>
        </tr>
        </tbody>
      </table>
    </div>

    <div class="modal fade" id="webAuthnDeleteModal" tabindex="-1" aria-labelledby="webAuthnDeleteModalLabel" aria-hidden="true">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header bg-danger text-white">
            <h5 class="modal-title" id="webAuthnDeleteModalLabel">{{ $t('settings.webauthn.modal-delete.headline') }}</h5>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" :aria-label="$t('settings.webauthn.modal-delete.button-cancel')"></button>
          </div>
          <div class="modal-body">
            <h5 class="mb-3">{{ selectedCredential.Name }} <small class="text-body-secondary">({{ $t('settings.webauthn.modal-delete.created') }} {{ selectedCredential.CreatedAt }})</small></h5>
            <p class="mb-0">{{ $t('settings.webauthn.modal-delete.abstract') }}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('settings.webauthn.modal-delete.button-cancel') }}</button>
            <button type="button" class="btn btn-danger" id="confirmWebAuthnDelete" @click="auth.DeleteWebAuthnCredential(selectedCredential.ID)" :disabled="auth.isFetching" data-bs-dismiss="modal">{{ $t('settings.webauthn.modal-delete.button-delete') }}</button>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>
