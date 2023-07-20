<script setup>
import Modal from "./Modal.vue";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import Vue3TagsInput from 'vue3-tags-input';
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
    return t("interfaces.interface.edit") + ": " + selectedInterface.value.Identifier
  }
  return t("interfaces.interface.new")
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

function handleChangePeerDefNetwork(tags) {
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
    formData.value.PeerDefNetwork = tags
  }
}

function handleChangePeerDefAllowedIPs(tags) {
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
    formData.value.PeerDefAllowedIPs = tags
  }
}

function handleChangePeerDefDns(tags) {
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
    formData.value.PeerDefDns = tags
  }
}

function handleChangePeerDefDnsSearch(tags) {
  formData.value.PeerDefDnsSearch = tags
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
      title: "Backend Connection Failure",
      text: "Failed to save interface!",
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
      title: "Backend Connection Failure",
      text: "Failed to apply peer defaults!",
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
      title: "Backend Connection Failure",
      text: "Failed to delete interface!",
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
          <a class="nav-link active" data-bs-toggle="tab" href="#interface">Interface</a>
        </li>
        <li v-if="formData.Mode==='server'" class="nav-item">
          <a class="nav-link" data-bs-toggle="tab" href="#peerdefaults">Peer Defaults</a>
        </li>
      </ul>
      <div id="interfaceTabs" class="tab-content">
        <div id="interface" class="tab-pane fade active show">
          <fieldset>
            <legend class="mt-4">General</legend>
            <div v-if="props.interfaceId==='#NEW#'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.identifier') }}</label>
              <input v-model="formData.Identifier" class="form-control" placeholder="The device identifier" type="text">
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.displayname') }}</label>
              <select v-model="formData.Mode" class="form-select">
                <option value="server">Server Mode</option>
                <option value="client">Client Mode</option>
                <option value="any">Custom Mode</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.displayname') }}</label>
              <input v-model="formData.DisplayName" class="form-control" placeholder="A descriptive name of the interface" type="text">
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">Cryptography</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.privatekey') }}</label>
              <input v-model="formData.PrivateKey" class="form-control" placeholder="The private key" required type="email">
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.publickey') }}</label>
              <input v-model="formData.PublicKey" class="form-control" placeholder="The public key" required type="email">
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">Networking</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.ips') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.Addresses"
                               placeholder="IP Addresses (CIDR format)"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateCIDR"
                               @on-tags-changed="handleChangeAddresses"/>
            </div>
            <div v-if="formData.Mode==='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.listenport') }}</label>
              <input v-model="formData.ListenPort" class="form-control" placeholder="Listen Port" type="number">
            </div>
            <div v-if="formData.Mode!=='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.dns') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.Dns"
                               placeholder="DNS Servers"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateIP"
                               @on-tags-changed="handleChangeDns"/>
            </div>
            <div v-if="formData.Mode!=='server'" class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.dnssearch') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.DnsSearch"
                               placeholder="DNS Search prefixes"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateDomain"
                               @on-tags-changed="handleChangeDnsSearch"/>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.mtu') }}</label>
                <input v-model="formData.Mtu" class="form-control" placeholder="Client MTU (0 = default)" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.firewallmark') }}</label>
                <input v-model="formData.FirewallMark" class="form-control" placeholder="Firewall Mark (0 = default)" type="number">
              </div>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.routingtable') }}</label>
                <input v-model="formData.RoutingTable" class="form-control" placeholder="Routing Table (0 = default)" type="number">
              </div>
              <div class="form-group col-md-6">
              </div>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">Hooks</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.preup') }}</label>
              <textarea v-model="formData.PreUp" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.postup') }}</label>
              <textarea v-model="formData.PostUp" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.predown') }}</label>
              <textarea v-model="formData.PreDown" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.postdown') }}</label>
              <textarea v-model="formData.PostDown" class="form-control" rows="2"></textarea>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">State</legend>
            <div class="form-check form-switch">
              <input v-model="formData.Disabled" class="form-check-input" type="checkbox">
              <label class="form-check-label" >Disabled</label>
            </div>
            <div class="form-check form-switch">
              <input v-model="formData.SaveConfig" checked="" class="form-check-input" type="checkbox">
              <label class="form-check-label">Save Config to File</label>
            </div>
          </fieldset>
        </div>
        <div id="peerdefaults" class="tab-pane fade">
          <fieldset>
            <legend class="mt-4">Networking</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.endpoint') }}</label>
              <input v-model="formData.PeerDefEndpoint" class="form-control" placeholder="Endpoint Addresses" type="text">
              <small class="form-text text-muted">The endpoint address that peers will connect to.</small>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.networks') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.PeerDefNetwork"
                               placeholder="Network Addresses"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateCIDR"
                               @on-tags-changed="handleChangePeerDefNetwork"/>
              <small class="form-text text-muted">Peers will get IP addresses from those subnets.</small>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.allowedips') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.PeerDefAllowedIPs"
                               placeholder="Default Allowed IP Addresses"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateCIDR"
                               @on-tags-changed="handleChangePeerDefAllowedIPs"/>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.dns') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.PeerDefDns"
                               placeholder="DNS Servers"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateIP"
                               @on-tags-changed="handleChangePeerDefDns"/>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.dnssearch') }}</label>
              <vue3-tags-input class="form-control" :tags="formData.PeerDefDnsSearch"
                               placeholder="DNS Search prefix"
                               :add-tag-on-keys="[13, 188, 32, 9]"
                               :validate="validateDomain"
                               @on-tags-changed="handleChangePeerDefDnsSearch"/>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.mtu') }}</label>
                <input v-model="formData.PeerDefMtu" class="form-control" placeholder="Client MTU (0 = default)" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.firewallmark') }}</label>
                <input v-model="formData.PeerDefFirewallMark" class="form-control" placeholder="Firewall Mark (0 = default)" type="number">
              </div>
            </div>
            <div class="row">
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.routingtable') }}</label>
                <input v-model="formData.PeerDefRoutingTable" class="form-control" placeholder="Routing Table (0 = default)" type="number">
              </div>
              <div class="form-group col-md-6">
                <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.keepalive') }}</label>
                <input v-model="formData.PeerDefPersistentKeepalive" class="form-control" placeholder="Persistent Keepalive (0 = default)" type="number">
              </div>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mt-4">Hooks</legend>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.preup') }}</label>
              <textarea v-model="formData.PeerDefPreUp" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.postup') }}</label>
              <textarea v-model="formData.PeerDefPostUp" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.predown') }}</label>
              <textarea v-model="formData.PeerDefPreDown" class="form-control" rows="2"></textarea>
            </div>
            <div class="form-group">
              <label class="form-label mt-4">{{ $t('modals.interfaceedit.defaults.postdown') }}</label>
              <textarea v-model="formData.PeerDefPostDown" class="form-control" rows="2"></textarea>
            </div>
          </fieldset>
          <fieldset v-if="props.interfaceId!=='#NEW#'" class="text-end">
            <hr class="mt-4">
            <button class="btn btn-primary me-1" type="button" @click.prevent="applyPeerDefaults">Apply Peer Defaults</button>
          </fieldset>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.interfaceId!=='#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">Delete</button>
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
.v3ti .v3ti-tag {
  background: #fff;
  color: #343a40;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 0;
}

.v3ti .v3ti-tag .v3ti-remove-tag {
  color: #343a40;
  transition: color .3s;
}

a.v3ti-remove-tag {
  cursor: pointer;
  text-decoration: none;
}
</style>
