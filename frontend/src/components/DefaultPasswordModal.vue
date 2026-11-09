<template>
  <Teleport to="#modals">
    <div v-if="visible" class="modal-backdrop fade show">
      <div class="modal fade show" tabindex="-1">
        <div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable" @click.stop="">
          <div class="modal-content">
            <div class="modal-header bg-warning text-dark">
              <h5 class="modal-title">
                <i class="fa-solid fa-triangle-exclamation me-2"></i>{{ $t('default-password.modal-title') }}
              </h5>
            </div>
            <div class="modal-body col-md-12">
              <div class="alert alert-warning">
                <h5 class="alert-heading">{{ $t('default-password.modal-warning-heading') }}</h5>
                <p>{{ $t('default-password.modal-warning-text') }}</p>
                <hr>
                <p class="mb-0 fw-bold">{{ $t('default-password.modal-warning-note') }}</p>
              </div>

              <form @submit.prevent="updatePassword">
                <div class="form-group mb-3">
                  <label class="form-label" for="default-oldpw">{{ $t('default-password.current-label') }}</label>
                  <input id="default-oldpw" v-model="pwFormData.OldPassword" class="form-control"
                         :class="{ 'is-invalid': pwFormData.Password && !pwFormData.OldPassword }" type="password" required>
                </div>

                <div class="form-group mb-3">
                  <label class="form-label" for="default-newpw">{{ $t('default-password.new-label') }}</label>
                  <input id="default-newpw" v-model="pwFormData.Password" class="form-control"
                         :class="{ 'is-invalid': passwordWeak || passwordSame, 'is-valid': pwFormData.Password !== '' && !passwordWeak && !passwordSame }"
                         type="password" required>
                  <div class="invalid-feedback" v-if="passwordWeak">{{ $t('default-password.weak-label') }}</div>
                  <div class="invalid-feedback" v-else-if="passwordSame">{{ $t('default-password.same-label') }}</div>
                </div>

                <div class="form-group mb-3">
                  <label class="form-label" for="default-confirmnewpw">{{ $t('default-password.confirm-label') }}</label>
                  <input id="default-confirmnewpw" v-model="pwFormData.PasswordRepeat" class="form-control"
                         :class="{ 'is-invalid': passwordMismatch, 'is-valid': pwFormData.PasswordRepeat !== '' && !passwordMismatch && !passwordWeak && !passwordSame }"
                         type="password" required>
                  <div class="invalid-feedback" v-if="passwordMismatch">{{ $t('default-password.mismatch-label') }}</div>
                </div>
              </form>
            </div>
            <div class="modal-footer">
              <button class="btn btn-primary" :disabled="submitting || !passwordChangeAllowed" @click.prevent="updatePassword">
                <i class="fa-solid fa-floppy-disk me-2"></i>{{ $t('default-password.change-button-text') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, ref } from "vue";
import { authStore } from "@/stores/auth";
import { profileStore } from "@/stores/profile";
import { settingsStore } from "@/stores/settings";
import { notify } from "@kyvg/vue3-notification";
import { useI18n } from "vue-i18n";

const auth = authStore();
const profile = profileStore();
const settings = settingsStore();
const { t } = useI18n();

const visible = computed(() => auth.IsAuthenticated && auth.DefaultPasswordInUse);

const pwFormData = ref({
  OldPassword: 'wgportal-default',
  Password: '',
  PasswordRepeat: '',
});

const submitting = ref(false);

const minPasswordLength = computed(() => settings.Setting('MinPasswordLength') || 16);

const passwordWeak = computed(() => {
  return pwFormData.value.Password && pwFormData.value.Password.length > 0 && pwFormData.value.Password.length < minPasswordLength.value;
});

const passwordMismatch = computed(() => {
  return pwFormData.value.PasswordRepeat !== '' && pwFormData.value.Password !== pwFormData.value.PasswordRepeat;
});

const passwordSame = computed(() => {
  return pwFormData.value.OldPassword && pwFormData.value.Password && pwFormData.value.OldPassword === pwFormData.value.Password;
});

const passwordChangeAllowed = computed(() => {
  return pwFormData.value.Password &&
      pwFormData.value.Password.length >= minPasswordLength.value &&
      !passwordMismatch.value &&
      pwFormData.value.OldPassword &&
      pwFormData.value.OldPassword.length > 0 &&
      !passwordSame.value;
});

const updatePassword = async () => {
  if (!passwordChangeAllowed.value || submitting.value) {
    return;
  }

  submitting.value = true;
  try {
    await profile.changePassword(pwFormData.value);
    auth.SetDefaultPasswordInUse(false);

    pwFormData.value.OldPassword = '';
    pwFormData.value.Password = '';
    pwFormData.value.PasswordRepeat = '';

    notify({
      title: t('default-password.success-title'),
      text: t('default-password.success-text'),
      type: 'success',
      duration: 5000,
    });

    await settings.LoadSettings();
    await profile.LoadUser();
  } catch (e) {
    notify({
      title: "Failed to update password!",
      text: e.toString(),
      type: 'error',
    });
  } finally {
    submitting.value = false;
  }
};
</script>

<style scoped>
.modal.show {
  display: block;
  opacity: 1.0;
}
.modal-backdrop {
  background-color: rgba(0, 0, 0, 0.7) !important;
  z-index: 1055;
}
.modal-backdrop.show {
  opacity: 1.0 !important;
}
</style>
