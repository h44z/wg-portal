<script setup>

import {ref} from "vue";

const privateKey = ref("")
const publicKey = ref("")
const presharedKey = ref("")

/**
 * Generate an X25519 keypair using the Web Crypto API and return Base64-encoded strings.
 * @async
 * @function generateKeypair
 * @returns {Promise<{ publicKey: string, privateKey: string }>} Resolves with an object containing
 *   - publicKey: the Base64-encoded public key
 *   - privateKey: the Base64-encoded private key
 */
async function generateKeypair() {
  // 1. Generate an X25519 key pair
  const keyPair = await crypto.subtle.generateKey(
      { name: 'X25519', namedCurve: 'X25519' },
      true,                 // extractable
      ['deriveBits']        // allowed usage for ECDH
  );

  // 2. Export keys as JWK to access raw key material
  const pubJwk  = await crypto.subtle.exportKey('jwk', keyPair.publicKey);
  const privJwk = await crypto.subtle.exportKey('jwk', keyPair.privateKey);

  // 3. Convert Base64URL to standard Base64 with padding
  return {
    publicKey:  b64urlToB64(pubJwk.x),
    privateKey: b64urlToB64(privJwk.d)
  };
}

/**
 * Generate a 32-byte pre-shared key using crypto.getRandomValues.
 * @function generatePresharedKey
 * @returns {Uint8Array} A Uint8Array of length 32 with random bytes.
 */
function generatePresharedKey() {
  let privateKey = new Uint8Array(32);
  window.crypto.getRandomValues(privateKey);
  return privateKey;
}

/**
 * Convert a Base64URL-encoded string to standard Base64 with padding.
 * @function b64urlToB64
 * @param {string} input - The Base64URL string.
 * @returns {string} The padded, standard Base64 string.
 */
function b64urlToB64(input) {
  let b64 = input.replace(/-/g, '+').replace(/_/g, '/');
  while (b64.length % 4) {
    b64 += '=';
  }
  return b64;
}

/**
 * Convert an ArrayBuffer or TypedArray buffer to a Base64-encoded string.
 * @function arrayBufferToBase64
 * @param {ArrayBuffer|Uint8Array} buffer - The buffer to convert.
 * @returns {string} Base64-encoded representation of the buffer.
 */
function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; ++i) {
    binary += String.fromCharCode(bytes[i]);
  }
  // Window.btoa handles binary → Base64
  return btoa(binary);
}

/**
 * Generate a new keypair and update the corresponding Vue refs.
 * @async
 * @function generateNewKeyPair
 * @returns {Promise<void>}
 */
async function generateNewKeyPair() {
  const keypair = await generateKeypair();

  privateKey.value = keypair.privateKey;
  publicKey.value = keypair.publicKey;
}

/**
 * Generate a new pre-shared key and update the Vue ref.
 * @function generateNewPresharedKey
 */
function generateNewPresharedKey() {
  const rawPsk = generatePresharedKey();
  presharedKey.value = arrayBufferToBase64(rawPsk);
}

</script>

<template>
  <div class="page-header">
    <h1>{{ $t('keygen.headline') }}</h1>
  </div>

  <p class="lead">{{ $t('keygen.abstract') }}</p>

  <div class="mt-4 row">
    <div class="col-12 col-lg-5">
      <h1>{{ $t('keygen.headline-keypair') }}</h1>
      <fieldset>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('keygen.private-key.label') }}</label>
          <input class="form-control" v-model="privateKey" :placeholder="$t('keygen.private-key.placeholder')" readonly>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('keygen.public-key.label') }}</label>
          <input class="form-control" v-model="publicKey" :placeholder="$t('keygen.private-key.placeholder')" readonly>
        </div>
      </fieldset>
      <fieldset>
        <hr class="mt-4">
        <button class="btn btn-primary mb-4" type="button" @click.prevent="generateNewKeyPair">{{ $t('keygen.button-generate') }}</button>
      </fieldset>
    </div>
    <div class="col-12 col-lg-2 mt-sm-4">
    </div>
    <div class="col-12 col-lg-5">
      <h1>{{ $t('keygen.headline-preshared-key') }}</h1>
      <fieldset>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('keygen.preshared-key.label') }}</label>
          <input class="form-control" v-model="presharedKey" :placeholder="$t('keygen.preshared-key.placeholder')" readonly>
        </div>
      </fieldset>
      <fieldset>
        <hr class="mt-4">
        <button class="btn btn-primary mb-4" type="button" @click.prevent="generateNewPresharedKey">{{ $t('keygen.button-generate') }}</button>
      </fieldset>
    </div>
  </div>

</template>

<style scoped>

</style>