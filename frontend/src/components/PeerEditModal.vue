<script setup>
import Modal from "./Modal.vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import Vue3TagsInput from "vue3-tags-input";
import { validateCIDR, validateIP, validateDomain } from '@/helpers/validators';
import isCidr from "is-cidr";
import {isIP} from 'is-ip';
import { freshPeer, freshInterface } from '@/helpers/models';

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
    i = freshInterface() // dummy interface to avoid 'undefined' exceptions
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

const formData = ref(freshPeer())

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        console.log(selectedInterface.value)
        console.log(selectedPeer.value)
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

        }
      }
    }
)

function close() {
  formData.value = freshPeer()
  emit('close')
}

function handleChangeAddresses(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.Addresses = tags
  }
}

function handleChangeAllowedIPs(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.AllowedIPs.Value = tags
  }
}

function handleChangeExtraAllowedIPs(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.ExtraAllowedIPs = tags
  }
}

function handleChangeDns(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(!isIP(tag)) {
      validInput = false
      notify({
        title: "Invalid IP",
        text: tag + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.Dns = tags
  }
}

function handleChangeDnsSearch(tags) {
  formData.value.DnsSearch = tags
}

async function save() {
  try {
    if (props.peerId!=='#NEW#') {
      await peers.UpdatePeer(selectedPeer.value.Identifier, formData.value)
    } else {
      await peers.CreatePeer(selectedInterface.value.Identifier, formData.value)
    }
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Backend Connection Failure",
      text: "Failed to save peer!",
      type: 'error',
    })
  }
}

async function del() {
  try {
    await peers.DeletePeer(selectedPeer.value.Identifier)
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Backend Connection Failure",
      text: "Failed to delete peer!",
      type: 'error',
    })
  }
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
        <div class="form-group" v-if="formData.Mode==='client'">
          <label class="form-label mt-4">{{ $t('modals.peeredit.endpointpublickey') }}</label>
          <input type="text" class="form-control" placeholder="Endpoint Public Key" v-model="formData.EndpointPublicKey.Value">
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
          <vue3-tags-input class="form-control" :tags="formData.Addresses"
                           placeholder="IP Addresses (CIDR format)"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           :validate="validateCIDR"
                           @on-tags-changed="handleChangeAddresses"/>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.allowedips') }}</label>
          <vue3-tags-input class="form-control" :tags="formData.AllowedIPs.Value"
                           placeholder="Allowed IP Addresses (CIDR format)"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           :validate="validateCIDR"
                           @on-tags-changed="handleChangeAllowedIPs"/>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.extraallowedips') }}</label>
          <vue3-tags-input class="form-control" :tags="formData.ExtraAllowedIPs"
                           placeholder="Extra allowed IP's (Server Sided)"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           :validate="validateCIDR"
                           @on-tags-changed="handleChangeExtraAllowedIPs"/>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.dns') }}</label>
          <vue3-tags-input class="form-control" :tags="formData.Dns.Value"
                           placeholder="DNS Servers"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           :validate="validateIP"
                           @on-tags-changed="handleChangeDns"/>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peeredit.dnssearch') }}</label>
          <vue3-tags-input class="form-control" :tags="formData.DnsSearch.Value"
                           placeholder="DNS Search prefixes"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           :validate="validateDomain"
                           @on-tags-changed="handleChangeDnsSearch"/>
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peeredit.persistendkeepalive') }}</label>
            <input type="number" class="form-control" placeholder="Persistent Keepalive (0 = off)" v-model="formData.PersistentKeepalive.Value">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.peeredit.mtu') }}</label>
            <input type="number" class="form-control" placeholder="Client MTU (0 = default)" v-model="formData.Mtu.Value">
          </div>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">State</legend>
        <div class="row">
          <div class="form-group col-md-6">
            <div class="form-check form-switch">
              <input class="form-check-input" type="checkbox" v-model="formData.Disabled">
              <label class="form-check-label" >Disabled</label>
            </div>
          </div>
          <div class="form-group col-md-6">
            <label class="form-label">{{ $t('modals.peeredit.expiresat') }}</label>
            <input type="date" pattern="\d{4}-\d{2}-\d{2}" class="form-control" min="2023-01-01" v-model="formData.ExpiresAt">
          </div>
        </div>
      </fieldset>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.peerId!=='#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">Delete</button>
      </div>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save">Save</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">Discard</button>
    </template>
  </Modal>
</template>

<style>
.config-qr-img {
  max-width: 100%;
}
</style>
