<script setup>
import Modal from "./Modal.vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import {useI18n} from "vue-i18n";
import {freshInterface, freshPeer, freshStats} from '@/helpers/models';
import Prism from "vue-prism-component";
import {notify} from "@kyvg/vue3-notification";
import {settingsStore} from "@/stores/settings";

const { t } = useI18n()

const settings = settingsStore()
const peers = peerStore()
const interfaces = interfaceStore()

const props = defineProps({
  peerId: String,
  visible: Boolean,
})

const emit = defineEmits(['close'])

function close() {
  emit('close')
}

const configString = ref("")

const selectedPeer = computed(() => {
  let p = peers.Find(props.peerId)

  if (!p) {
    p = freshPeer() // dummy peer to avoid 'undefined' exceptions
  }

  return p
})

const selectedStats = computed(() => {
  let s = peers.Statistics(props.peerId)

  if (!s) {
    s = freshStats() // dummy peer to avoid 'undefined' exceptions
  }

  return s
})

const selectedInterface = computed(() => {
  let i = interfaces.GetSelected;

  if (!i) {
    i = freshInterface() // dummy interface to avoid 'undefined' exceptions
  }

  return i
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }
  if (selectedInterface.value.Mode === "server") {
    return t("modals.peer-view.headline-peer") + " " + selectedPeer.value.DisplayName
  } else {
    return t("modals.peer-view.headline-endpoint") + " " + selectedPeer.value.DisplayName
  }
})

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        await peers.LoadPeerConfig(selectedPeer.value.Identifier)
        configString.value = peers.configuration
      }
    }
)

function download() {
  // credit: https://www.bitdegree.org/learn/javascript-download
  let filename = 'WireGuard-Tunnel.conf'
  if (selectedPeer.value.DisplayName) {
    filename = selectedPeer.value.DisplayName
        .replace(/ /g,"_")
        .replace(/[^a-zA-Z0-9-_]/g,"")
        .substring(0, 16)
        + ".conf"
  }
  let text = configString.value

  let element = document.createElement('a')
  element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(text))
  element.setAttribute('download', filename)

  element.style.display = 'none'
  document.body.appendChild(element)

  element.click()
  document.body.removeChild(element)
}

function email() {
  peers.MailPeerConfig(settings.Setting("MailLinkOnly"), [selectedPeer.value.Identifier]).catch(e => {
    notify({
      title: "Failed to send mail with peer configuration!",
      text: e.toString(),
      type: 'error',
    })
  })
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <div class="accordion" id="peerInformation">
        <div class="accordion-item">
          <h2 class="accordion-header">
            <button class="accordion-button" type="button" data-bs-toggle="collapse" data-bs-target="#collapseDetails" aria-expanded="true" aria-controls="collapseDetails">
              {{ $t('modals.peer-view.section-info') }}
            </button>
          </h2>
          <div id="collapseDetails" class="accordion-collapse collapse show" aria-labelledby="headingDetails" data-bs-parent="#peerInformation" style="">
            <div class="accordion-body">
              <div class="row">
                <div class="col-md-8">
                  <ul>
                    <li>{{ $t('modals.peer-view.identifier') }}: {{ selectedPeer.PublicKey }}</li>
                    <li>{{ $t('modals.peer-view.ip') }}: <span v-for="ip in selectedPeer.Addresses" :key="ip" class="badge rounded-pill bg-light">{{ ip }}</span></li>
                    <li>{{ $t('modals.peer-view.user') }}: {{ selectedPeer.UserIdentifier }}</li>
                    <li v-if="selectedPeer.Notes">{{ $t('modals.peer-view.notes') }}: {{ selectedPeer.Notes }}</li>
                    <li v-if="selectedPeer.ExpiresAt">{{ $t('modals.peer-view.expiry-status') }}: {{ selectedPeer.ExpiresAt }}</li>
                    <li v-if="selectedPeer.Disabled">{{ $t('modals.peer-view.disabled-status') }}: {{ selectedPeer.DisabledReason }}</li>
                  </ul>
                </div>
                <div class="col-md-4">
                  <img class="config-qr-img" :src="peers.ConfigQrUrl(props.peerId)" loading="lazy" alt="Configuration QR Code">
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="accordion-item">
          <h2 class="accordion-header" id="headingStatus">
            <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#collapseStatus" aria-expanded="false" aria-controls="collapseStatus">
              {{ $t('modals.peer-view.section-status') }}
            </button>
          </h2>
          <div id="collapseStatus" class="accordion-collapse collapse" aria-labelledby="headingStatus" data-bs-parent="#peerInformation" style="">
            <div class="accordion-body">
              <div class="row">
                <div class="col-md-12">
                  <h4>{{ $t('modals.peer-view.traffic') }}</h4>
                  <p><i class="fas fa-long-arrow-alt-down" :title="$t('modals.peer-view.download')"></i> {{ selectedStats.BytesReceived }} Bytes / <i class="fas fa-long-arrow-alt-up" :title="$t('modals.peer-view.upload')"></i> {{ selectedStats.BytesTransmitted }} Bytes</p>
                  <h4>{{ $t('modals.peer-view.connection-status') }}</h4>
                  <ul>
                    <li>{{ $t('modals.peer-view.pingable') }}: {{ selectedStats.IsPingable }}</li>
                    <li>{{ $t('modals.peer-view.handshake') }}: {{ selectedStats.LastHandshake }}</li>
                    <li>{{ $t('modals.peer-view.connected-since') }}: {{ selectedStats.LastSessionStart }}</li>
                    <li>{{ $t('modals.peer-view.endpoint') }}: {{ selectedStats.EndpointAddress }}</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-if="selectedInterface.Mode==='server'" class="accordion-item">
          <h2 class="accordion-header" id="headingConfig">
            <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#collapseConfig" aria-expanded="false" aria-controls="collapseConfig">
              {{ $t('modals.peer-view.section-config') }}
            </button>
          </h2>
          <div id="collapseConfig" class="accordion-collapse collapse" aria-labelledby="headingConfig" data-bs-parent="#peerInformation" style="">
            <div class="accordion-body">
              <Prism language="ini" :code="configString"></Prism>
            </div>
          </div>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button @click.prevent="download" type="button" class="btn btn-primary me-1">{{ $t('modals.peer-view.button-download') }}</button>
        <button @click.prevent="email"  type="button" class="btn btn-primary me-1">{{ $t('modals.peer-view.button-email') }}</button>
      </div>
      <button @click.prevent="close" type="button" class="btn btn-secondary">{{ $t('general.close') }}</button>


    </template>
  </Modal>
</template>

<style>
.config-qr-img {
  max-width: 100%;
}
</style>
