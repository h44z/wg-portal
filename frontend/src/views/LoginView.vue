<script setup>

import {computed, onMounted, ref} from "vue";
import {authStore} from "@/stores/auth";
import router from '../router/index.js'
import {notify} from "@kyvg/vue3-notification";
import {settingsStore} from "@/stores/settings";

const auth = authStore()
const settings = settingsStore()

const loggingIn = ref(false)
const username = ref("")
const password = ref("")

const usernameInvalid = computed(() => username.value === "")
const passwordInvalid = computed(() => password.value === "")
const disableLoginBtn = computed(() => username.value === "" || password.value === "" || loggingIn.value)


onMounted(async () => {
  await settings.LoadSettings()
})

const login = async function () {
  console.log("Performing login for user:", username.value);
  loggingIn.value = true;
  auth.Login(username.value, password.value)
      .then(uid => {
        notify({
          title: "Logged in",
          text: "Authentication succeeded!",
          type: 'success',
        });
        loggingIn.value = false;
        settings.LoadSettings(); // reload full settings
        router.push(auth.ReturnUrl);
      })
      .catch(error => {
        notify({
          title: "Login failed!",
          text: "Authentication failed!",
          type: 'error',
        });

        //loggingIn.value = false;
        // delay the user from logging in for a short amount of time
        setTimeout(() => loggingIn.value = false, 1000);
      });
}

const loginWebAuthn = async function () {
  console.log("Performing webauthn login");
  loggingIn.value = true;
  auth.LoginWebAuthn()
      .then(uid => {
        notify({
          title: "Logged in",
          text: "Authentication succeeded!",
          type: 'success',
        });
        loggingIn.value = false;
        settings.LoadSettings(); // reload full settings
        router.push(auth.ReturnUrl);
      })
      .catch(error => {
        notify({
          title: "Login failed!",
          text: "Authentication failed!",
          type: 'error',
        });

        //loggingIn.value = false;
        // delay the user from logging in for a short amount of time
        setTimeout(() => loggingIn.value = false, 1000);
      });
}

const externalLogin = function (provider) {
  console.log("Performing external login for provider", provider.Identifier);
  loggingIn.value = true;
  console.log(router.currentRoute.value);
  let currentUri = window.location.origin + "/#" + router.currentRoute.value.fullPath;
  let redirectUrl = `${WGPORTAL_BACKEND_BASE_URL}${provider.ProviderUrl}`;
  redirectUrl += "?redirect=true";
  redirectUrl += "&return=" + encodeURIComponent(currentUri);
  window.location.href = redirectUrl;
}
</script>

<template>
  <div class="row">
    <div class="col-lg-3"></div><!-- left spacer -->
    <div class="col-lg-6">
      <div class="card mt-5">
        <div class="card-header">{{ $t('login.headline') }}<div class="float-end">
          <RouterLink :to="{ name: 'home' }" class="nav-link" :title="$t('menu.home')"><i class="fas fa-times-circle"></i></RouterLink>
        </div></div>
        <div class="card-body">
          <form method="post">
            <fieldset>
              <div class="form-group">
                <label class="form-label" for="inputUsername">{{ $t('login.username.label') }}</label>
                <div class="input-group mb-3">
                  <span class="input-group-text"><span class="far fa-user p-2"></span></span>
                  <input id="inputUsername" v-model="username" :class="{'is-invalid':usernameInvalid, 'is-valid':!usernameInvalid}" :placeholder="$t('login.username.placeholder')" aria-describedby="usernameHelp"
                         class="form-control"
                         name="username" type="text">
                </div>
              </div>
              <div class="form-group">
                <label class="form-label" for="inputPassword">{{ $t('login.password.label') }}</label>
                <div class="input-group mb-3">
                  <span class="input-group-text"><span class="fas fa-lock p-2"></span></span>
                  <input id="inputPassword" v-model="password" :class="{'is-invalid':passwordInvalid, 'is-valid':!passwordInvalid}" :placeholder="$t('login.password.placeholder')" class="form-control"
                         name="password" type="password">
                </div>
              </div>

              <div class="row mt-5 mb-2">
                <div class="col-lg-4">
                  <button :disabled="disableLoginBtn" class="btn btn-primary" type="submit" @click.prevent="login">
                    {{ $t('login.button') }} <div v-if="loggingIn" class="d-inline"><i class="ms-2 fa-solid fa-circle-notch fa-spin"></i></div>
                  </button>
                </div>
                <div class="col-lg-8 mb-2 text-end">
                  <button v-if="settings.Setting('WebAuthnEnabled')" class="btn btn-primary" type="submit" @click.prevent="loginWebAuthn">
                    {{ $t('login.button-webauthn') }} <div v-if="loggingIn" class="d-inline"><i class="ms-2 fa-solid fa-circle-notch fa-spin"></i></div>
                  </button>
                </div>
              </div>

              <div class="row mt-5 d-flex">
                <div class="col-lg-12 d-flex mb-2">
                  <!-- OpenIdConnect / OAUTH providers -->
                  <button v-for="(provider, idx) in auth.LoginProviders" :key="provider.Identifier" :class="{'ms-1':idx > 0}"
                          :disabled="loggingIn" :title="provider.Name" class="btn btn-outline-primary flex-fill"
                          v-html="provider.Name" @click.prevent="externalLogin(provider)"></button>
                </div>
              </div>

              <div class="mt-3">
              </div>
            </fieldset>
          </form>


        </div>
      </div>
    </div>
    <div class="col-lg-3"></div><!-- right spacer -->
  </div>
</template>
