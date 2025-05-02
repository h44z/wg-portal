<script setup>

import {ref} from "vue";

const privateKey = ref("")
const publicKey = ref("")
const presharedKey = ref("")

// region wireguard.js
// taken from: https://github.com/WireGuard/wireguard-tools/blob/master/contrib/keygen-html/wireguard.js

function gf(init) {
  let r = new Float64Array(16);
  if (init) {
    for (let i = 0; i < init.length; ++i)
      r[i] = init[i];
  }
  return r;
}

function pack(o, n) {
  let b, m = gf(), t = gf();
  for (let i = 0; i < 16; ++i)
    t[i] = n[i];
  carry(t);
  carry(t);
  carry(t);
  for (let j = 0; j < 2; ++j) {
    m[0] = t[0] - 0xffed;
    for (let i = 1; i < 15; ++i) {
      m[i] = t[i] - 0xffff - ((m[i - 1] >> 16) & 1);
      m[i - 1] &= 0xffff;
    }
    m[15] = t[15] - 0x7fff - ((m[14] >> 16) & 1);
    b = (m[15] >> 16) & 1;
    m[14] &= 0xffff;
    cswap(t, m, 1 - b);
  }
  for (let i = 0; i < 16; ++i) {
    o[2 * i] = t[i] & 0xff;
    o[2 * i + 1] = t[i] >> 8;
  }
}

function carry(o) {
  for (let i = 0; i < 16; ++i) {
    o[(i + 1) % 16] += (i < 15 ? 1 : 38) * Math.floor(o[i] / 65536);
    o[i] &= 0xffff;
  }
}

function cswap(p, q, b) {
  let t, c = ~(b - 1);
  for (let i = 0; i < 16; ++i) {
    t = c & (p[i] ^ q[i]);
    p[i] ^= t;
    q[i] ^= t;
  }
}

function add(o, a, b) {
  for (let i = 0; i < 16; ++i)
    o[i] = (a[i] + b[i]) | 0;
}

function subtract(o, a, b) {
  for (let i = 0; i < 16; ++i)
    o[i] = (a[i] - b[i]) | 0;
}

function multmod(o, a, b) {
  let t = new Float64Array(31);
  for (let i = 0; i < 16; ++i) {
    for (let j = 0; j < 16; ++j)
      t[i + j] += a[i] * b[j];
  }
  for (let i = 0; i < 15; ++i)
    t[i] += 38 * t[i + 16];
  for (let i = 0; i < 16; ++i)
    o[i] = t[i];
  carry(o);
  carry(o);
}

function invert(o, i) {
  let c = gf();
  for (let a = 0; a < 16; ++a)
    c[a] = i[a];
  for (let a = 253; a >= 0; --a) {
    multmod(c, c, c);
    if (a !== 2 && a !== 4)
      multmod(c, c, i);
  }
  for (let a = 0; a < 16; ++a)
    o[a] = c[a];
}

function clamp(z) {
  z[31] = (z[31] & 127) | 64;
  z[0] &= 248;
}

function generatePublicKey(privateKey) {
  let r, z = new Uint8Array(32);
  let a = gf([1]),
      b = gf([9]),
      c = gf(),
      d = gf([1]),
      e = gf(),
      f = gf(),
      _121665 = gf([0xdb41, 1]),
      _9 = gf([9]);
  for (let i = 0; i < 32; ++i)
    z[i] = privateKey[i];
  clamp(z);
  for (let i = 254; i >= 0; --i) {
    r = (z[i >>> 3] >>> (i & 7)) & 1;
    cswap(a, b, r);
    cswap(c, d, r);
    add(e, a, c);
    subtract(a, a, c);
    add(c, b, d);
    subtract(b, b, d);
    multmod(d, e, e);
    multmod(f, a, a);
    multmod(a, c, a);
    multmod(c, b, e);
    add(e, a, c);
    subtract(a, a, c);
    multmod(b, a, a);
    subtract(c, d, f);
    multmod(a, c, _121665);
    add(a, a, d);
    multmod(c, c, a);
    multmod(a, d, f);
    multmod(d, b, _9);
    multmod(b, e, e);
    cswap(a, b, r);
    cswap(c, d, r);
  }
  invert(c, c);
  multmod(a, a, c);
  pack(z, a);
  return z;
}

function generatePresharedKey() {
  let privateKey = new Uint8Array(32);
  window.crypto.getRandomValues(privateKey);
  return privateKey;
}

function generatePrivateKey() {
  let privateKey = generatePresharedKey();
  clamp(privateKey);
  return privateKey;
}

function encodeBase64(dest, src) {
  let input = Uint8Array.from([(src[0] >> 2) & 63, ((src[0] << 4) | (src[1] >> 4)) & 63, ((src[1] << 2) | (src[2] >> 6)) & 63, src[2] & 63]);
  for (let i = 0; i < 4; ++i)
    dest[i] = input[i] + 65 +
        (((25 - input[i]) >> 8) & 6) -
        (((51 - input[i]) >> 8) & 75) -
        (((61 - input[i]) >> 8) & 15) +
        (((62 - input[i]) >> 8) & 3);
}

function keyToBase64(key) {
  let i, base64 = new Uint8Array(44);
  for (i = 0; i < 32 / 3; ++i)
    encodeBase64(base64.subarray(i * 4), key.subarray(i * 3));
  encodeBase64(base64.subarray(i * 4), Uint8Array.from([key[i * 3 + 0], key[i * 3 + 1], 0]));
  base64[43] = 61;
  return String.fromCharCode.apply(null, base64);
}

// endregion wireguard.js

function generateNewKeyPair() {
  const priv = generatePrivateKey();
  const pub = generatePublicKey(priv);
  
  privateKey.value = keyToBase64(priv);
  publicKey.value = keyToBase64(pub);
}

function generateNewPresharedKey() {
  const psk = generatePresharedKey();
  presharedKey.value = keyToBase64(psk);
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