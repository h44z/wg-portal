<script setup>
import Modal from "./Modal.vue";
import { peerStore } from "@/stores/peers";
import { computed, ref, watch } from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import { freshPeer, freshInterface } from '@/helpers/models';
import { profileStore } from "@/stores/profile";

const { t } = useI18n()

const peers = peerStore()
const profile = profileStore()

const props = defineProps({
  peerId: String,
  visible: Boolean,
})

const emit = defineEmits(['close'])

const selectedPeer = computed(() => {
  let p = peers.Find(props.peerId)

  if (!p) {
    if (!!props.peerId || props.peerId.length) {
      p = profile.peers.find((p) => p.Identifier === props.peerId)
    } else {
      p = freshPeer() // dummy peer to avoid 'undefined' exceptions
    }
  }
  return p
})

const selectedInterface = computed(() => {
  let iId = profile.selectedInterfaceId;

  let i = freshInterface() // dummy interface to avoid 'undefined' exceptions
  if (iId) {
    i = profile.interfaces.find((i) => i.Identifier === iId)
  }

  return i
})

const title = computed(() => {
  if (!props.visible) {
    return ""
  }

  if (selectedPeer.value) {
    return t("modals.peer-edit.headline-edit-peer") + " " + selectedPeer.value.Identifier
  }
  return t("modals.peer-edit.headline-new-peer")
})

const formData = ref(freshPeer())

// functions

watch(() => props.visible, async (newValue, oldValue) => {
  if (oldValue === false && newValue === true) { // if modal is shown
    if (!selectedPeer.value) {
      await peers.PreparePeer(selectedInterface.value.Identifier)

      formData.value.Identifier = peers.Prepared.Identifier
      formData.value.DisplayName = peers.Prepared.DisplayName
      formData.value.UserIdentifier = peers.Prepared.UserIdentifier
      formData.value.InterfaceIdentifier = peers.Prepared.InterfaceIdentifier
      formData.value.Disabled = peers.Prepared.Disabled
      formData.value.ExpiresAt = peers.Prepared.ExpiresAt
      formData.value.Notes = peers.Prepared.Notes

      formData.value.Endpoint = peers.Prepared.Endpoint
      formData.value.EndpointPublicKey = peers.Prepared.EndpointPublicKey
      formData.value.AllowedIPs = peers.Prepared.AllowedIPs
      formData.value.ExtraAllowedIPs = peers.Prepared.ExtraAllowedIPs
      formData.value.PresharedKey = peers.Prepared.PresharedKey
      formData.value.PersistentKeepalive = peers.Prepared.PersistentKeepalive

      formData.value.PrivateKey = peers.Prepared.PrivateKey
      formData.value.PublicKey = peers.Prepared.PublicKey

      formData.value.Mode = peers.Prepared.Mode

      formData.value.Addresses = peers.Prepared.Addresses
      formData.value.CheckAliveAddress = peers.Prepared.CheckAliveAddress
      formData.value.Dns = peers.Prepared.Dns
      formData.value.DnsSearch = peers.Prepared.DnsSearch
      formData.value.Mtu = peers.Prepared.Mtu
      formData.value.FirewallMark = peers.Prepared.FirewallMark
      formData.value.RoutingTable = peers.Prepared.RoutingTable

      formData.value.PreUp = peers.Prepared.PreUp
      formData.value.PostUp = peers.Prepared.PostUp
      formData.value.PreDown = peers.Prepared.PreDown
      formData.value.PostDown = peers.Prepared.PostDown

    } else { // fill existing data
      formData.value.Identifier = selectedPeer.value.Identifier
      formData.value.DisplayName = selectedPeer.value.DisplayName
      formData.value.UserIdentifier = selectedPeer.value.UserIdentifier
      formData.value.InterfaceIdentifier = selectedPeer.value.InterfaceIdentifier
      formData.value.Disabled = selectedPeer.value.Disabled
      formData.value.ExpiresAt = selectedPeer.value.ExpiresAt
      formData.value.Notes = selectedPeer.value.Notes

      formData.value.Endpoint = selectedPeer.value.Endpoint
      formData.value.EndpointPublicKey = selectedPeer.value.EndpointPublicKey
      formData.value.AllowedIPs = selectedPeer.value.AllowedIPs
      formData.value.ExtraAllowedIPs = selectedPeer.value.ExtraAllowedIPs
      formData.value.PresharedKey = selectedPeer.value.PresharedKey
      formData.value.PersistentKeepalive = selectedPeer.value.PersistentKeepalive

      formData.value.PrivateKey = selectedPeer.value.PrivateKey
      formData.value.PublicKey = selectedPeer.value.PublicKey

      formData.value.Mode = selectedPeer.value.Mode

      formData.value.Addresses = selectedPeer.value.Addresses
      formData.value.CheckAliveAddress = selectedPeer.value.CheckAliveAddress
      formData.value.Dns = selectedPeer.value.Dns
      formData.value.DnsSearch = selectedPeer.value.DnsSearch
      formData.value.Mtu = selectedPeer.value.Mtu
      formData.value.FirewallMark = selectedPeer.value.FirewallMark
      formData.value.RoutingTable = selectedPeer.value.RoutingTable

      formData.value.PreUp = selectedPeer.value.PreUp
      formData.value.PostUp = selectedPeer.value.PostUp
      formData.value.PreDown = selectedPeer.value.PreDown
      formData.value.PostDown = selectedPeer.value.PostDown

      if (!formData.value.Endpoint.Overridable ||
        !formData.value.EndpointPublicKey.Overridable ||
        !formData.value.AllowedIPs.Overridable ||
        !formData.value.PersistentKeepalive.Overridable ||
        !formData.value.Dns.Overridable ||
        !formData.value.DnsSearch.Overridable ||
        !formData.value.Mtu.Overridable ||
        !formData.value.FirewallMark.Overridable ||
        !formData.value.RoutingTable.Overridable ||
        !formData.value.PreUp.Overridable ||
        !formData.value.PostUp.Overridable ||
        !formData.value.PreDown.Overridable ||
        !formData.value.PostDown.Overridable) {
        formData.value.IgnoreGlobalSettings = true
      }
    }
  }
}
)

watch(() => formData.value.Disabled, async (newValue, oldValue) => {
  if (oldValue && !newValue && formData.value.ExpiresAt) {
    formData.value.ExpiresAt = "" // reset expiry date
  }
}
)

function close() {
  formData.value = freshPeer()
  emit('close')
}

async function save() {
  try {
    if (props.peerId !== '#NEW#') {
      await peers.UpdatePeer(selectedPeer.value.Identifier, formData.value)
    } else {
      await peers.CreatePeer(selectedInterface.value.Identifier, formData.value)
    }
    close()
  } catch (e) {
    // console.log(e)
    notify({
      title: "Failed to save peer!",
      text: e.toString(),
      type: 'error',
    })
  }
}

async function del() {
  try {
    await peers.DeletePeer(selectedPeer.value.Identifier)
    close()
  } catch (e) {
    // console.log(e)
    notify({
      title: "Failed to delete peer!",
      text: e.toString(),
      type: 'error',
    })
  }
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.peer-edit.header-general') }}</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.display-name.label') }}</label>
          <input type="text" class="form-control" :placeholder="$t('modals.peer-edit.display-name.placeholder')"
            v-model="formData.DisplayName">
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.peer-edit.header-crypto') }}</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.private-key.label') }}</label>
          <input type="text" class="form-control" :placeholder="$t('modals.peer-edit.private-key.placeholder')" required
            v-model="formData.PrivateKey">
          <small id="privateKeyHelp" class="form-text text-muted">{{ $t('modals.peer-edit.private-key.help') }}</small>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.public-key.label') }}</label>
          <input type="text" class="form-control" :placeholder="$t('modals.peer-edit.public-key.placeholder')" required
            v-model="formData.PublicKey">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.preshared-key.label') }}</label>
          <input type="text" class="form-control" :placeholder="$t('modals.peer-edit.preshared-key.placeholder')"
            v-model="formData.PresharedKey">
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.peer-edit.header-network') }}</legend>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peer-edit.keep-alive.label') }}</label>
            <input type="number" class="form-control" :placeholder="$t('modals.peer-edit.keep-alive.label')"
              v-model="formData.PersistentKeepalive.Value">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peer-edit.mtu.label') }}</label>
            <input type="number" class="form-control" :placeholder="$t('modals.peer-edit.mtu.label')"
              v-model="formData.Mtu.Value">
          </div>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.peer-edit.header-hooks') }}</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.pre-up.label') }}</label>
          <textarea v-model="formData.PreUp.Value" class="form-control" rows="2"
            :placeholder="$t('modals.peer-edit.pre-up.placeholder')"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.post-up.label') }}</label>
          <textarea v-model="formData.PostUp.Value" class="form-control" rows="2"
            :placeholder="$t('modals.peer-edit.post-up.placeholder')"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.pre-down.label') }}</label>
          <textarea v-model="formData.PreDown.Value" class="form-control" rows="2"
            :placeholder="$t('modals.peer-edit.pre-down.placeholder')"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-edit.post-down.label') }}</label>
          <textarea v-model="formData.PostDown.Value" class="form-control" rows="2"
            :placeholder="$t('modals.peer-edit.post-down.placeholder')"></textarea>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.peer-edit.header-state') }}</legend>
        <div class="row">
          <div class="form-group col-md-6">
            <div class="form-check form-switch">
              <input class="form-check-input" type="checkbox" v-model="formData.Disabled">
              <label class="form-check-label">{{ $t('modals.peer-edit.disabled.label') }}</label>
            </div>
          </div>
          <div class="form-group col-md-6">
            <label class="form-label">{{ $t('modals.peer-edit.expires-at.label') }}</label>
            <input type="date" pattern="\d{4}-\d{2}-\d{2}" class="form-control" min="2023-01-01"
              v-model="formData.ExpiresAt">
          </div>
        </div>
      </fieldset>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.peerId !== '#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">{{
          $t('general.delete') }}</button>
      </div>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save">{{ $t('general.save') }}</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">{{ $t('general.close') }}</button>
    </template>
  </Modal>
</template>

<style></style>
