<script setup>
import { onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { RouterLink } from "vue-router";
import { useI18n } from "vue-i18n";
import { notify } from "@kyvg/vue3-notification";
import { authStore } from "@/stores/auth";
import { peerStore } from "@/stores/peers";
import { base64_url_decode } from "@/helpers/encoding";

const { t } = useI18n()

const route = useRoute()
const peers = peerStore()

const inProgress = ref(true)
const failed = ref(false)

let peerId = ""
let configStyle = "wgquick"

function triggerBrowserDownload(filename, text) {
  // credit: https://www.bitdegree.org/learn/javascript-download
  let element = document.createElement('a')
  element.setAttribute('href', 'data:application/octet-stream;charset=utf-8,' + encodeURIComponent(text))
  element.setAttribute('download', filename)

  element.style.display = 'none'
  document.body.appendChild(element)

  element.click()
  document.body.removeChild(element)
}

async function startDownload() {
  inProgress.value = true
  failed.value = false

  try {
    // Loading the peer gives us access to the correct filename. Access rights are enforced by the backend,
    // so only the owner (or an administrator) is able to load the peer and its configuration.
    await peers.LoadPeer(peerId)
    await peers.LoadPeerConfig(peerId, configStyle)

    const config = peers.configuration
    if (!config) {
      throw new Error("empty configuration")
    }

    const filename = (peers.peer && peers.peer.Filename) ? peers.peer.Filename : "WireGuard-Tunnel.conf"
    triggerBrowserDownload(filename, config)

    notify({
      title: t('peer-config-download.success-title'),
      text: t('peer-config-download.success-message'),
      type: 'success',
    })
    inProgress.value = false
  } catch (e) {
    console.error("Failed to download peer configuration:", e)
    failed.value = true
    inProgress.value = false
    const auth = authStore()
    if (auth.IsAuthenticated) {
      notify({
        title: t('peer-config-download.error-title'),
        text: t('peer-config-download.error-message'),
        type: 'error',
      })
    }
  }
}

onMounted(async () => {
  const rawId = route.params.id
  try {
    peerId = base64_url_decode(rawId)
  } catch (e) {
    peerId = rawId // fall back to the raw value if it is not base64-url encoded
  }

  const styleParam = route.query.style
  if (styleParam === "wgquick" || styleParam === "raw") {
    configStyle = styleParam
  }

  await startDownload()
})
</script>

<template>
  <div class="page-header">
    <h1>{{ $t('peer-config-download.headline') }}</h1>
  </div>

  <div class="card border-secondary p-5 text-center">
    <div v-if="inProgress">
      <div class="spinner-border text-primary mb-3" role="status">
        <span class="visually-hidden">...</span>
      </div>
      <p class="lead">{{ $t('peer-config-download.in-progress') }}</p>
    </div>
    <div v-else>
      <p class="lead">{{ failed ? $t('peer-config-download.error-message') : $t('peer-config-download.success-message') }}</p>
      <p class="card-text">{{ $t('peer-config-download.manual-hint') }}</p>
      <div class="mt-3">
        <button type="button" class="btn btn-primary me-2" @click.prevent="startDownload">{{ $t('peer-config-download.button-retry') }}</button>
        <RouterLink :to="{ name: 'home' }" class="btn btn-secondary">{{ $t('peer-config-download.button-home') }}</RouterLink>
      </div>
    </div>
  </div>
</template>
