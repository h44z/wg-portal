<script setup>
import Modal from "./Modal.vue";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import { VueTagsInput } from '@vojtechlanka/vue-tags-input';
import { validateCIDR, validateIP, validateDomain } from '@/helpers/validators';
import isCidr from "is-cidr";
import {isIP} from 'is-ip';
import { freshInterface } from '@/helpers/models';
import {peerStore} from "@/stores/peers";

const { t } = useI18n()

const interfaces = interfaceStore()
const peers = peerStore()

const props = defineProps({
  interfaceId: String,
  visible: Boolean,
})

const emit = defineEmits(['close'])

const selectedInterface = computed(() => {
  return interfaces.Find(props.interfaceId)
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }

  if (selectedInterface.value) {
    return t("modals.interface-edit.headline-edit") + " " + selectedInterface.value.Identifier
  }
  return t("modals.interface-edit.headline-new")
})

const currentTags = ref({
  Addresses: "",
  Dns: "",
  DnsSearch: "",
  PeerDefNetwork: "",
  PeerDefAllowedIPs: "",
  PeerDefDns: "",
  PeerDefDnsSearch: ""
})
const formData = ref(freshInterface())

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        console.log(selectedInterface.value)
        if (!selectedInterface.value) {
          await interfaces.PrepareInterface()

          // fill form data
          formData.value.Identifier = interfaces.Prepared.Identifier
          formData.value.DisplayName = interfaces.Prepared.DisplayName
          formData.value.Mode = interfaces.Prepared.Mode

          formData.value.PublicKey = interfaces.Prepared.PublicKey
          formData.value.PrivateKey = interfaces.Prepared.PrivateKey

          formData.value.ListenPort = interfaces.Prepared.ListenPort
          formData.value.Addresses = interfaces.Prepared.Addresses
          formData.value.Dns = interfaces.Prepared.Dns
          formData.value.DnsSearch = interfaces.Prepared.DnsSearch

          formData.value.Mtu = interfaces.Prepared.Mtu
          formData.value.FirewallMark = interfaces.Prepared.FirewallMark
          formData.value.RoutingTable = interfaces.Prepared.RoutingTable

          formData.value.PreUp = interfaces.Prepared.PreUp
          formData.value.PostUp = interfaces.Prepared.PostUp
          formData.value.PreDown = interfaces.Prepared.PreDown
          formData.value.PostDown = interfaces.Prepared.PostDown

          formData.value.SaveConfig = interfaces.Prepared.SaveConfig

          formData.value.PeerDefNetwork = interfaces.Prepared.PeerDefNetwork
          formData.value.PeerDefDns = interfaces.Prepared.PeerDefDns
          formData.value.PeerDefDnsSearch = interfaces.Prepared.PeerDefDnsSearch
          formData.value.PeerDefEndpoint = interfaces.Prepared.PeerDefEndpoint
          formData.value.PeerDefAllowedIPs = interfaces.Prepared.PeerDefAllowedIPs
          formData.value.PeerDefMtu = interfaces.Prepared.PeerDefMtu
          formData.value.PeerDefPersistentKeepalive = interfaces.Prepared.PeerDefPersistentKeepalive
          formData.value.PeerDefFirewallMark = interfaces.Prepared.PeerDefFirewallMark
          formData.value.PeerDefRoutingTable = interfaces.Prepared.PeerDefRoutingTable
          formData.value.PeerDefPreUp = interfaces.Prepared.PeerDefPreUp
          formData.value.PeerDefPostUp = interfaces.Prepared.PeerDefPostUp
          formData.value.PeerDefPreDown = interfaces.Prepared.PeerDefPreDown
          formData.value.PeerDefPostDown = interfaces.Prepared.PeerDefPostDown
        } else { // fill existing userdata
          formData.value.Disabled = selectedInterface.value.Disabled
          formData.value.Identifier = selectedInterface.value.Identifier
          formData.value.DisplayName = selectedInterface.value.DisplayName
          formData.value.Mode = selectedInterface.value.Mode

          formData.value.PublicKey = selectedInterface.value.PublicKey
          formData.value.PrivateKey = selectedInterface.value.PrivateKey

          formData.value.ListenPort = selectedInterface.value.ListenPort
          formData.value.Addresses = selectedInterface.value.Addresses
          formData.value.Dns = selectedInterface.value.Dns
          formData.value.DnsSearch = selectedInterface.value.DnsSearch

          formData.value.Mtu = selectedInterface.value.Mtu
          formData.value.FirewallMark = selectedInterface.value.FirewallMark
          formData.value.RoutingTable = selectedInterface.value.RoutingTable

          formData.value.PreUp = selectedInterface.value.PreUp
          formData.value.PostUp = selectedInterface.value.PostUp
          formData.value.PreDown = selectedInterface.value.PreDown
          formData.value.PostDown = selectedInterface.value.PostDown

          formData.value.SaveConfig = selectedInterface.value.SaveConfig

          formData.value.PeerDefNetwork = selectedInterface.value.PeerDefNetwork
          formData.value.PeerDefDns = selectedInterface.value.PeerDefDns
          formData.value.PeerDefDnsSearch = selectedInterface.value.PeerDefDnsSearch
          formData.value.PeerDefEndpoint = selectedInterface.value.PeerDefEndpoint
          formData.value.PeerDefAllowedIPs = selectedInterface.value.PeerDefAllowedIPs
          formData.value.PeerDefMtu = selectedInterface.value.PeerDefMtu
          formData.value.PeerDefPersistentKeepalive = selectedInterface.value.PeerDefPersistentKeepalive
          formData.value.PeerDefFirewallMark = selectedInterface.value.PeerDefFirewallMark
          formData.value.PeerDefRoutingTable = selectedInterface.value.PeerDefRoutingTable
          formData.value.PeerDefPreUp = selectedInterface.value.PeerDefPreUp
          formData.value.PeerDefPostUp = selectedInterface.value.PeerDefPostUp
          formData.value.PeerDefPreDown = selectedInterface.value.PeerDefPreDown
          formData.value.PeerDefPostDown = selectedInterface.value.PeerDefPostDown

        }
      }
    }
)

function close() {
  formData.value = freshInterface()
  emit('close')
}

function handleChangeAddresses(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag.text) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag.text + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.Addresses = tags.map(tag => tag.text)
  }
}

function handleChangeDns(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(!isIP(tag.text)) {
      validInput = false
      notify({
        title: "Invalid IP",
        text: tag.text + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.Dns = tags.map(tag => tag.text)
  }
}

function handleChangeDnsSearch(tags) {
  formData.value.DnsSearch = tags.map(tag => tag.text)
}

function handleChangePeerDefNetwork(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag.text) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag.text + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.PeerDefNetwork = tags.map(tag => tag.text)
  }
}

function handleChangePeerDefAllowedIPs(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(isCidr(tag.text) === 0) {
      validInput = false
      notify({
        title: "Invalid CIDR",
        text: tag.text + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.PeerDefAllowedIPs = tags.map(tag => tag.text)
  }
}

function handleChangePeerDefDns(tags) {
  let validInput = true
  tags.forEach(tag => {
    if(!isIP(tag.text)) {
      validInput = false
      notify({
        title: "Invalid IP",
        text: tag.text + " is not a valid IP address",
        type: 'error',
      })
    }
  })
  if(validInput) {
    formData.value.PeerDefDns = tags.map(tag => tag.text)
  }
}

function handleChangePeerDefDnsSearch(tags) {
  formData.value.PeerDefDnsSearch = tags.map(tag => tag.text)
}

async function save() {
  try {
    if (props.interfaceId!=='#NEW#') {
      await interfaces.UpdateInterface(selectedInterface.value.Identifier, formData.value)
    } else {
      await interfaces.CreateInterface(formData.value)
    }
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Failed to save interface!",
      text: e.toString(),
      type: 'error',
    })
  }
}

async function applyPeerDefaults() {
  if (props.interfaceId==='#NEW#') {
    return; // do nothing for new interfaces
  }

  try {
    await interfaces.ApplyPeerDefaults(selectedInterface.value.Identifier, formData.value)

    notify({
      title: "Peer Defaults Applied",
      text: "Applied current peer defaults to all available peers.",
      type: 'success',
    })

    await peers.LoadPeers(selectedInterface.value.Identifier) // reload all peers after applying the defaults
  } catch (e) {
    console.log(e)
    notify({
      title: "Failed to apply peer defaults!",
      text: e.toString(),
      type: 'error',
    })
  }
}

async function del() {
  try {
    await interfaces.DeleteInterface(selectedInterface.value.Identifier)
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Failed to delete interface!",
      text: e.toString(),
      type: 'error',
    })
  }
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <ul class="nav nav-tabs">
        <li class="nav-item">
          <a class="nav-link active" data-bs-toggle="tab" href="#interface">{{ $t('modals.interface-edit.tab-interface') }}</a>
        </li>
        <li v-if="formData.Mode==='server'" class="nav-item">
          <a class="nav-link" data-bs-toggle="tab" href="#peerdefaults">{{ $t('modals.interface-edit.tab-peerdef') }}</a>
        </li>
      </ul>
      <div id="interfaceTabs" class="tab-content">
        <div id="interface" class="tab-pane fade active show">
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-general') }}</legend>
            <div v-if="props.interfaceId==='#NEW#'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.identifier.label') }}</label>
              <input v-model="formData.Identifier" class="form-control" :placeholder="$t('modals.interface-edit.identifier.placeholder')" type="text">
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.mode.label') }}</label>
              <select v-model="formData.Mode" class="form-select">
                <option value="server">{{ $t('modals.interface-edit.mode.server') }}</option>
                <option value="client">{{ $t('modals.interface-edit.mode.client') }}</option>
                <option value="any">{{ $t('modals.interface-edit.mode.any') }}</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.display-name.label') }}</label>
              <input v-model="formData.DisplayName" class="form-control" :placeholder="$t('modals.interface-edit.display-name.placeholder')" type="text">
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-crypto') }}</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.private-key.label') }}</label>
              <input v-model="formData.PrivateKey" class="form-control" :placeholder="$t('modals.interface-edit.private-key.placeholder')" required type="text">
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.public-key.label') }}</label>
              <input v-model="formData.PublicKey" class="form-control" :placeholder="$t('modals.interface-edit.public-key.placeholder')" required type="text">
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-network') }}</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.ip.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.Addresses"
                              :tags="formData.Addresses.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.ip.placeholder')"
                              :validation="validateCIDR()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangeAddresses"/>
            </div>
            <div v-if="formData.Mode==='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.listen-port.label') }}</label>
              <input v-model="formData.ListenPort" class="form-control" :placeholder="$t('modals.interface-edit.listen-port.placeholder')" type="number">
            </div>
            <div v-if="formData.Mode!=='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.dns.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.Dns"
                              :tags="formData.Dns.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.dns.placeholder')"
                              :validation="validateIP()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangeDns"/>
            </div>
            <div v-if="formData.Mode!=='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.dns-search.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.DnsSearch"
                              :tags="formData.DnsSearch.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.dns-search.placeholder')"
                              :validation="validateDomain()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangeDnsSearch"/>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.mtu.label') }}</label>
                <input v-model="formData.Mtu" class="form-control" :placeholder="$t('modals.interface-edit.mtu.placeholder')" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.firewall-mark.label') }}</label>
                <input v-model="formData.FirewallMark" class="form-control" :placeholder="$t('modals.interface-edit.firewall-mark.placeholder')" type="number">
              </div>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.routing-table.label') }}</label>
                <input v-model="formData.RoutingTable" aria-describedby="routingTableHelp" class="form-control" :placeholder="$t('modals.interface-edit.routing-table.placeholder')" type="text">
                <small id="routingTableHelp" class="form-text text-muted">{{ $t('modals.interface-edit.routing-table.description') }}</small>
              </div>
              <div class="form-group col-md-6">
              </div>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-hooks') }}</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.pre-up.label') }}</label>
              <textarea v-model="formData.PreUp" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.pre-up.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.post-up.label') }}</label>
              <textarea v-model="formData.PostUp" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.post-up.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.pre-down.label') }}</label>
              <textarea v-model="formData.PreDown" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.pre-down.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.post-down.label') }}</label>
              <textarea v-model="formData.PostDown" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.post-down.placeholder')"></textarea>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-state') }}</legend>
            <div class="form-check form-switch">
              <input v-model="formData.Disabled" class="form-check-input" type="checkbox">
              <label class="form-check-label">{{ $t('modals.interface-edit.disabled.label') }}</label>
            </div>
            <div class="form-check form-switch">
              <input v-model="formData.SaveConfig" checked="" class="form-check-input" type="checkbox">
              <label class="form-check-label">{{ $t('modals.interface-edit.save-config.label') }}</label>
            </div>
          </fieldset>
        </div>
        <div id="peerdefaults" class="tab-pane fade">
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-network') }}</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.defaults.endpoint.label') }}</label>
              <input v-model="formData.PeerDefEndpoint" class="form-control" :placeholder="$t('modals.interface-edit.defaults.endpoint.placeholder')" type="text">
              <small class="form-text text-muted">{{ $t('modals.interface-edit.defaults.endpoint.description') }}</small>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.defaults.networks.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.PeerDefNetwork"
                              :tags="formData.PeerDefNetwork.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.defaults.networks.placeholder')"
                              :validation="validateCIDR()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangePeerDefNetwork"/>
              <small class="form-text text-muted">{{ $t('modals.interface-edit.defaults.networks.description') }}</small>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.defaults.allowed-ip.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.PeerDefAllowedIPs"
                              :tags="formData.PeerDefAllowedIPs.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.defaults.allowed-ip.placeholder')"
                              :validation="validateCIDR()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangePeerDefAllowedIPs"/>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.dns.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.PeerDefDns"
                              :tags="formData.PeerDefDns.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.dns.placeholder')"
                              :validation="validateIP()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangePeerDefDns"/>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.dns-search.label') }}</label>
              <vue-tags-input class="form-control" v-model="currentTags.PeerDefDnsSearch"
                              :tags="formData.PeerDefDnsSearch.map(str => ({ text: str }))"
                              :placeholder="$t('modals.interface-edit.dns-search.placeholder')"
                              :validation="validateDomain()"
                              :add-on-key="[13, 188, 32, 9]"
                              :save-on-key="[13, 188, 32, 9]"
                              :allow-edit-tags="true"
                              :separators="[',', ';', ' ']"
                              @tags-changed="handleChangePeerDefDnsSearch"/>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.defaults.mtu.label') }}</label>
                <input v-model="formData.PeerDefMtu" class="form-control" :placeholder="$t('modals.interface-edit.defaults.mtu.placeholder')" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.firewall-mark.label') }}</label>
                <input v-model="formData.PeerDefFirewallMark" class="form-control" :placeholder="$t('modals.interface-edit.firewall-mark.placeholder')" type="number">
              </div>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.routing-table.label') }}</label>
                <input v-model="formData.PeerDefRoutingTable" class="form-control" :placeholder="$t('modals.interface-edit.routing-table.placeholder')" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interface-edit.defaults.keep-alive.label') }}</label>
                <input v-model="formData.PeerDefPersistentKeepalive" class="form-control" :placeholder="$t('modals.interface-edit.defaults.keep-alive.placeholder')" type="number">
              </div>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">{{ $t('modals.interface-edit.header-peer-hooks') }}</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.pre-up.label') }}</label>
              <textarea v-model="formData.PeerDefPreUp" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.pre-up.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.post-up.label') }}</label>
              <textarea v-model="formData.PeerDefPostUp" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.post-up.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.pre-down.label') }}</label>
              <textarea v-model="formData.PeerDefPreDown" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.pre-down.placeholder')"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interface-edit.post-down.label') }}</label>
              <textarea v-model="formData.PeerDefPostDown" class="form-control" rows="2" :placeholder="$t('modals.interface-edit.post-down.placeholder')"></textarea>
            </div>
          </fieldset>
          <fieldset v-if="props.interfaceId!=='#NEW#'" class="text-end">
            <hr class="mt-4">
            <button class="btn btn-primary me-1" type="button" @click.prevent="applyPeerDefaults">{{ $t('modals.interface-edit.button-apply-defaults') }}</button>
          </fieldset>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.interfaceId!=='#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">{{ $t('general.delete') }}</button>
      </div>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save">{{ $t('general.save') }}</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">{{ $t('general.close') }}</button>
    </template>
  </Modal>
</template>

<style>

</style>
