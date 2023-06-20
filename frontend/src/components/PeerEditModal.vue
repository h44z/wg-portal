<script setup>
import Modal from "./Modal.vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";

const { t } = useI18n()

const peers = peerStore()
const interfaces = interfaceStore()

const props = defineProps({
  peerId: String,
  visible: Boolean,
})

const emit = defineEmits(['close'])

const selectedPeer = computed(() => {
  return peers.Find(props.peerId)
})

const selectedInterface = computed(() => {
  let i = interfaces.GetSelected;

  if (!i) {
    i = { // dummy interface to avoid 'undefined' exceptions
      Identifier: "none",
      Mode: "server"
    }
  }

  return i
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }
  if (selectedInterface.value.Mode === "server") {
    if (selectedPeer.value) {
      return t("interfaces.peer.edit") + ": " + selectedPeer.value.Name
    }
    return t("interfaces.peer.new")
  } else {
    if (selectedPeer.value) {
      return t("interfaces.endpoint.edit") + ": " + selectedPeer.value.Name
    }
    return t("interfaces.endpoint.new")
  }
})

const formData = ref(freshFormData())


function freshFormData() {
  return {
    Disabled: false,
    IgnoreGlobalSettings: true,

    Endpoint: {
      Value: "",
      Overridable: false,
    },
    AllowedIPsStr: {
      Value: "",
      Overridable: false,
    },
    ExtraAllowedIPsStr: "",
    PrivateKey: "",
    PublicKey: "",
    PresharedKey: "",
    PersistentKeepalive: {
      Value: 0,
      Overridable: false,
    },

    DisplayName: "",
    Identifier: "",
    UserIdentifier: "",

    InterfaceConfig: {
      PublicKey: {
        Value: "",
        Overridable: false,
      },
      AddressStr: {
        Value: "",
        Overridable: false,
      },
      DnsStr: {
        Value: "",
        Overridable: false,
      },
      DnsSearchStr: {
        Value: "",
        Overridable: false,
      },
      Mtu: {
        Value: 0,
        Overridable: false,
      },
      FirewallMark: {
        Value: 0,
        Overridable: false,
      },
      RoutingTable: {
        Value: "",
        Overridable: false,
      },
      PreUp: {
        Value: "",
        Overridable: false,
      },
      PostUp: {
        Value: "",
        Overridable: false,
      },
      PreDown: {
        Value: "",
        Overridable: false,
      },
      PostDown: {
        Value: "",
        Overridable: false,
      },
    }
  }
}

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        console.log(selectedInterface.value)
        console.log(selectedPeer.value)
        if (!selectedPeer.value) {
          await peers.PreparePeer(selectedInterface.value.Identifier)

          formData.value.Disabled = peers.Prepared.Disabled
          formData.value.Identifier = peers.Prepared.Identifier
          formData.value.DisplayName = peers.Prepared.DisplayName

        } else { // fill existing data
          formData.value.Disabled = selectedPeer.value.Disabled
          formData.value.Identifier = selectedPeer.value.Identifier
          formData.value.DisplayName = selectedPeer.value.DisplayName

        }
      }
    }
)

function close() {
  formData.value = freshFormData()
  emit('close')
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <fieldset>
        <legend class="mt-4">General</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.displayname') }}</label>
          <input type="text" class="form-control" placeholder="A descriptive name of the peer" v-model="formData.DisplayName">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.linkeduser') }}</label>
          <input type="text" class="form-control" placeholder="Linked user" v-model="formData.UserIdentifier">
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">Cryptography</legend>
        <div class="form-group" v-if="selectedInterface.Mode==='server'">
          <label class="form-label mt-4">{{ $t('modals.peeredit.privatekey') }}</label>
          <input type="email" class="form-control" placeholder="The private key" required v-model="formData.PrivateKey">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.publickey') }}</label>
          <input type="email" class="form-control" placeholder="The public key" required v-model="formData.PublicKey">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.presharedkey') }}</label>
          <input type="email" class="form-control" placeholder="Optional pre-shared key" v-model="formData.PresharedKey">
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">Networking</legend>
        <div class="form-group" v-if="selectedInterface.Mode==='client'">
          <label class="form-label mt-4">{{ $t('modals.peeredit.endpoint') }}</label>
          <input type="text" class="form-control" placeholder="Endpoint Address" v-model="formData.Endpoint.Value">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.ips') }}</label>
          <input type="text" class="form-control" placeholder="Client IP Address" v-model="formData.InterfaceConfig.AddressStr.Value">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.allowedips') }}</label>
          <input type="text" class="form-control" placeholder="Allowed IP Address" v-model="formData.AllowedIPsStr.Value">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.extraallowedips') }}</label>
          <input type="text" class="form-control" placeholder="Extra Allowed IP's (Server Sided)" v-model="formData.ExtraAllowedIPsStr.Value">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.dns') }}</label>
          <input type="text" class="form-control" placeholder="Client DNS Servers" v-model="formData.InterfaceConfig.DnsStr.Value">
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peeredit.persistendkeepalive') }}</label>
            <input type="number" class="form-control" placeholder="Persistent Keepalive (0 = off)" v-model="formData.PersistentKeepalive.Value">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peeredit.mtu') }}</label>
            <input type="number" class="form-control" placeholder="Client MTU (0 = default)" v-model="formData.InterfaceConfig.Mtu.Value">
          </div>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">State</legend>
        <div class="form-check form-switch">
          <input class="form-check-input" type="checkbox" v-model="formData.Disabled">
          <label class="form-check-label" >Disabled</label>
        </div>
        <div class="form-check form-switch">
          <input class="form-check-input" type="checkbox" checked="" v-model="formData.IgnoreGlobalSettings">
          <label class="form-check-label">Ignore global settings</label>
        </div>
      </fieldset>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button type="button" class="btn btn-danger me-1">Delete</button>
      </div>
      <button type="button" class="btn btn-primary me-1">Save</button>
      <button @click.prevent="close" type="button" class="btn btn-secondary">Discard</button>
    </template>
  </Modal>
</template>

<style>
.config-qr-img {
  max-width: 100%;
}
</style>
