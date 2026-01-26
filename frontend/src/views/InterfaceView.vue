<script setup>
import PeerViewModal from "../components/PeerViewModal.vue";
import PeerEditModal from "../components/PeerEditModal.vue";
import PeerMultiCreateModal from "../components/PeerMultiCreateModal.vue";
import InterfaceEditModal from "../components/InterfaceEditModal.vue";
import InterfaceViewModal from "../components/InterfaceViewModal.vue";

import {computed, onMounted, ref} from "vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {notify} from "@kyvg/vue3-notification";
import {settingsStore} from "@/stores/settings";
import {humanFileSize} from '@/helpers/utils';
import {useI18n} from "vue-i18n";

const settings = settingsStore()
const interfaces = interfaceStore()
const peers = peerStore()

const { t } = useI18n()

const viewedPeerId = ref("")
const editPeerId = ref("")
const multiCreatePeerId = ref("")
const editInterfaceId = ref("")
const viewedInterfaceId = ref("")

const sortKey = ref("")
const sortOrder = ref(1)
const selectAll = ref(false)

const selectedPeers = computed(() => {
  return peers.All.filter(peer => peer.IsSelected).map(peer => peer.Identifier);
})

function sortBy(key) {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value * -1; // Toggle sort order
  } else {
    sortKey.value = key;
    sortOrder.value = 1; // Default to ascending
  }
  peers.sortKey = sortKey.value;
  peers.sortOrder = sortOrder.value;
}

function calculateInterfaceName(id, name) {
  let result = id
  if (name) {
    result += ' (' + name + ')'
  }
  return result
}

const calculateBackendName = computed(() => {
  let backendId = interfaces.GetSelected.Backend

  let backendName = t('interfaces.interface.unknown-backend')
  let availableBackends = settings.Setting('AvailableBackends') || []
  availableBackends.forEach(backend => {
    if (backend.Id === backendId) {
      backendName = backend.Id === 'local' ? t(backend.Name) : backend.Name
    }
  })
  return backendName
})

const isBackendValid = computed(() => {
  let backendId = interfaces.GetSelected.Backend

  let valid = false
  let availableBackends = settings.Setting('AvailableBackends') || []
  availableBackends.forEach(backend => {
    if (backend.Id === backendId) {
      valid = true
    }
  })
  return valid
})


async function download() {
  await interfaces.LoadInterfaceConfig(interfaces.GetSelected.Identifier)

  // credit: https://www.bitdegree.org/learn/javascript-download
  let text = interfaces.configuration

  let element = document.createElement('a')
  element.setAttribute('href', 'data:application/octet-stream;charset=utf-8,' + encodeURIComponent(text))
  element.setAttribute('download', interfaces.GetSelected.Filename)

  element.style.display = 'none'
  document.body.appendChild(element)

  element.click()
  document.body.removeChild(element)
}

async function saveConfig() {
  try {
    await interfaces.SaveConfiguration(interfaces.GetSelected.Identifier)

    notify({
      title: "Interface configuration persisted to file",
      text: "The interface configuration has been written to the wg-quick configuration file.",
      type: 'success',
    })
  } catch (e) {
    console.log(e)
    notify({
      title: "Failed to persist interface configuration file!",
      text: e.toString(),
      type: 'error',
    })
  }
}

async function bulkDelete() {
  if (confirm(t('interfaces.confirm-bulk-delete', {count: selectedPeers.value.length}))) {
    try {
      await peers.BulkDelete(selectedPeers.value)
      selectAll.value = false // reset selection
    } catch (e) {
      // notification is handled in store
    }
  }
}

async function bulkEnable() {
  try {
    await peers.BulkEnable(selectedPeers.value)
    selectAll.value = false
    peers.All.forEach(p => p.IsSelected = false) // remove selection
  } catch (e) {
    // notification is handled in store
  }
}

async function bulkDisable() {
  if (confirm(t('interfaces.confirm-bulk-disable', {count: selectedPeers.value.length}))) {
    try {
      await peers.BulkDisable(selectedPeers.value)
      selectAll.value = false
      peers.All.forEach(p => p.IsSelected = false) // remove selection
    } catch (e) {
      // notification is handled in store
    }
  }
}

function toggleSelectAll() {
  peers.FilteredAndPaged.forEach(peer => {
    peer.IsSelected = selectAll.value;
  });
}

onMounted(async () => {
  await interfaces.LoadInterfaces()
  await peers.LoadPeers(undefined) // use default interface
  await peers.LoadStats(undefined) // use default interface
})
</script>

<template>
  <PeerViewModal :peerId="viewedPeerId" :visible="viewedPeerId!==''" @close="viewedPeerId=''"></PeerViewModal>
  <PeerEditModal :peerId="editPeerId" :visible="editPeerId!==''" @close="editPeerId=''"></PeerEditModal>
  <PeerMultiCreateModal :visible="multiCreatePeerId!==''" @close="multiCreatePeerId=''"></PeerMultiCreateModal>
  <InterfaceEditModal :interfaceId="editInterfaceId" :visible="editInterfaceId!==''" @close="editInterfaceId=''"></InterfaceEditModal>
  <InterfaceViewModal :interfaceId="viewedInterfaceId" :visible="viewedInterfaceId!==''" @close="viewedInterfaceId=''"></InterfaceViewModal>

  <!-- Headline and interface selector -->
  <div class="page-header row">
    <div class="col-12 col-lg-8">
      <h1>{{ $t('interfaces.headline') }}</h1>
    </div>
    <div class="col-12 col-lg-4 text-end">
      <div class="form-group">

      </div>
      <div class="form-group">
        <div class="input-group mb-3">
          <button class="btn btn-primary" :title="$t('interfaces.button-add-interface')" @click.prevent="editInterfaceId='#NEW#'">
            <i class="fa-solid fa-plus-circle"></i>
          </button>
          <select v-model="interfaces.selected" :disabled="interfaces.Count===0" class="form-select" @change="() => { peers.LoadPeers(); peers.LoadStats() }">
            <option v-if="interfaces.Count===0" value="nothing">{{ $t('interfaces.no-interface.default-selection') }}</option>
            <option v-for="iface in interfaces.All" :key="iface.Identifier" :value="iface.Identifier">{{ calculateInterfaceName(iface.Identifier,iface.DisplayName) }}</option>
          </select>
        </div>
      </div>
    </div>
  </div>

  <!-- No interfaces information -->
  <div v-if="interfaces.Count===0" class="row">
    <div class="col-lg-12">
      <div class="mt-5">
        <h4>{{ $t('interfaces.no-interface.headline') }}</h4>
        <p>{{ $t('interfaces.no-interface.abstract') }}</p>
      </div>
    </div>
  </div>

  <!-- Interface overview -->
  <div v-if="interfaces.Count!==0" class="row">
    <div class="col-lg-12">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">
          <div class="row">
            <div class="col-12 col-lg-8">
              {{ $t('interfaces.interface.headline') }} <strong>{{interfaces.GetSelected.Identifier}}</strong> ({{ $t('modals.interface-edit.mode.' + interfaces.GetSelected.Mode )}} | {{ $t('interfaces.interface.backend') + ": " + calculateBackendName }}<span v-if="!isBackendValid" :title="t('interfaces.interface.wrong-backend')" class="ms-1 me-1"><i class="fa-solid fa-triangle-exclamation"></i></span>)
              <span v-if="interfaces.GetSelected.Disabled" class="text-danger"><i class="fa fa-circle-xmark" :title="interfaces.GetSelected.DisabledReason"></i></span>
              <div v-if="interfaces.GetSelected && (interfaces.TrafficStats.Received > 0 || interfaces.TrafficStats.Transmitted > 0)" class="mt-2">
                <small class="text-muted">
                  Traffic: <i class="fa-solid fa-arrow-down me-1"></i>{{ humanFileSize(interfaces.TrafficStats.Received) }}/s
                    <i class="fa-solid fa-arrow-up ms-1 me-1"></i>{{ humanFileSize(interfaces.TrafficStats.Transmitted) }}/s
                </small>
              </div>
            </div>
            <div class="col-12 col-lg-4 text-lg-end">
              <a class="btn-link" href="#" :title="$t('interfaces.interface.button-show-config')" @click.prevent="viewedInterfaceId=interfaces.GetSelected.Identifier"><i class="fas fa-eye"></i></a>
              <a class="ms-5 btn-link" href="#" :title="$t('interfaces.interface.button-download-config')" @click.prevent="download"><i class="fas fa-download"></i></a>
              <a v-if="settings.Setting('PersistentConfigSupported')" class="ms-5 btn-link" href="#" :title="$t('interfaces.interface.button-store-config')" @click.prevent="saveConfig"><i class="fas fa-save"></i></a>
              <a class="ms-5 btn-link" href="#" :title="$t('interfaces.interface.button-edit')" @click.prevent="editInterfaceId=interfaces.GetSelected.Identifier"><i class="fas fa-cog"></i></a>
            </div>
          </div>
        </div>
        <div class="card-body d-flex flex-column">
          <div v-if="interfaces.GetSelected.Mode==='server'" class="row">
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.key') }}:</td>
                  <td class="text-wrap">{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.endpoint') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefEndpoint}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.port') }}:</td>
                  <td>{{interfaces.GetSelected.ListenPort}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.peers') }}:</td>
                  <td>{{interfaces.GetSelected.EnabledPeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.total-peers') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Addresses" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.mtu') }}:</td>
                  <td>{{interfaces.GetSelected.Mtu}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.default-dns') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.PeerDefDns" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.default-keep-alive') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefPersistentKeepalive}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.default-allowed-ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.PeerDefAllowedIPs" :key="addr">{{addr}}</span></td>
                </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div v-if="interfaces.GetSelected.Mode==='client'" class="row">
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.key') }}:</td>
                  <td class="text-wrap">{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.endpoints') }}:</td>
                  <td>{{interfaces.GetSelected.EnabledPeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.total-endpoints') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Addresses" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.dns') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Dns" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.mtu') }}:</td>
                  <td>{{interfaces.GetSelected.Mtu}}</td>
                </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div v-if="interfaces.GetSelected.Mode==='any'" class="row">
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.key') }}:</td>
                  <td class="text-wrap">{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.endpoint') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefEndpoint}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.port') }}:</td>
                  <td>{{interfaces.GetSelected.ListenPort}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.peers') }}:</td>
                  <td>{{interfaces.GetSelected.EnabledPeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.total-peers') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.interface.ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Addresses" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.default-allowed-ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.PeerDefAllowedIPs" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.dns') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.PeerDefDns" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.mtu') }}:</td>
                  <td>{{interfaces.GetSelected.Mtu}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.interface.default-keep-alive') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefPersistentKeepalive}}</td>
                </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Peer list -->
  <div v-if="interfaces.Count!==0" class="mt-4 row">
    <div class="col-12 col-lg-5">
      <h2 v-if="interfaces.GetSelected.Mode==='server'" class="mt-2">{{ $t('interfaces.headline-peers') }}</h2>
      <h2 v-else class="mt-2">{{ $t('interfaces.headline-endpoints') }}</h2>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="peers.filter" class="form-control" :placeholder="$t('general.search.placeholder')" type="text" @keyup="peers.afterPageSizeChange">
          <button class="btn btn-primary" :title="$t('general.search.button')"><i class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-3 text-lg-end">
      <a class="btn btn-primary ms-2" href="#" :title="$t('interfaces.button-add-peers')" @click.prevent="multiCreatePeerId='#NEW#'"><i class="fa fa-plus me-1"></i><i class="fa fa-users"></i></a>
      <a class="btn btn-primary ms-2" href="#" :title="$t('interfaces.button-add-peer')" @click.prevent="editPeerId='#NEW#'"><i class="fa fa-plus me-1"></i><i class="fa fa-user"></i></a>
    </div>
  </div>
  <div class="row" v-if="selectedPeers.length > 0">
    <div class="col-12 text-lg-end">
      <a class="btn btn-outline-primary btn-sm ms-2" href="#" :title="$t('interfaces.button-bulk-enable')" @click.prevent="bulkEnable"><i class="fa-regular fa-circle-check"></i></a>
      <a class="btn btn-outline-primary btn-sm ms-2" href="#" :title="$t('interfaces.button-bulk-disable')" @click.prevent="bulkDisable"><i class="fa fa-ban"></i></a>
      <a class="btn btn-outline-danger btn-sm ms-2" href="#" :title="$t('interfaces.button-bulk-delete')" @click.prevent="bulkDelete"><i class="fa fa-trash-can"></i></a>
    </div>
  </div>
  <div v-if="interfaces.Count!==0" class="mt-2 table-responsive">
    <div v-if="peers.Count===0">
    <h4>{{ $t('interfaces.no-peer.headline') }}</h4>
    <p>{{ $t('interfaces.no-peer.abstract') }}</p>
    </div>
    <table v-if="peers.Count!==0" id="peerTable" class="table table-sm">
      <thead>
      <tr>
        <th scope="col">
          <input class="form-check-input" :title="$t('general.select-all')" type="checkbox" v-model="selectAll" @change="toggleSelectAll">
        </th><!-- select -->
        <th scope="col"></th><!-- status -->
        <th scope="col" @click="sortBy('DisplayName')">
          {{ $t("interfaces.table-heading.name") }}
          <i v-if="sortKey === 'DisplayName'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
        </th>
        <th scope="col" @click="sortBy('UserIdentifier')">
          {{ $t("interfaces.table-heading.user") }}
          <i v-if="sortKey === 'UserIdentifier'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
        </th>
        <th scope="col" @click="sortBy('Addresses')">
          {{ $t("interfaces.table-heading.ip") }}
          <i v-if="sortKey === 'Addresses'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
        </th>
        <th v-if="interfaces.GetSelected.Mode === 'client'" scope="col">
          {{ $t("interfaces.table-heading.endpoint") }}
        </th>
        <th v-if="peers.hasStatistics" scope="col" @click="sortBy('IsConnected')">
          {{ $t("interfaces.table-heading.status") }}
          <i v-if="sortKey === 'IsConnected'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
        </th>
        <th v-if="peers.hasStatistics" scope="col" @click="sortBy('Traffic')">RX/TX
          <i v-if="sortKey === 'Traffic'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
        </th>
        <th scope="col"></th><!-- Actions -->
      </tr>
      </thead>
      <tbody>
        <tr v-for="peer in peers.FilteredAndPaged" :key="peer.Identifier">
          <th scope="row">
            <input class="form-check-input" type="checkbox" v-model="peer.IsSelected">
          </th>
          <td class="text-center">
            <span v-if="peer.Disabled" class="text-danger" :title="$t('interfaces.peer-disabled') + ' ' + peer.DisabledReason"><i class="fa fa-circle-xmark"></i></span>
            <span v-if="!peer.Disabled && peer.ExpiresAt" class="text-warning" :title="$t('interfaces.peer-expiring') + ' ' +  peer.ExpiresAt"><i class="fas fa-hourglass-end expiring-peer"></i></span>
          </td>
          <td><span v-if="peer.DisplayName" :title="peer.Identifier">{{peer.DisplayName}}</span><span v-else :title="peer.Identifier">{{ $filters.truncate(peer.Identifier, 10)}}</span></td>
          <td><span :title="peer.UserDisplayName">{{peer.UserIdentifier}}</span></td>
          <td>
            <span v-for="ip in peer.Addresses" :key="ip" class="badge bg-light me-1">{{ ip }}</span>
          </td>
          <td v-if="interfaces.GetSelected.Mode==='client'">{{peer.Endpoint.Value}}</td>
          <td v-if="peers.hasStatistics">
            <div v-if="peers.Statistics(peer.Identifier).IsConnected">
              <span class="badge rounded-pill bg-success" :title="$t('interfaces.peer-connected')"><i class="fa-solid fa-link"></i></span> <small class="text-muted" :title="$t('interfaces.peer-handshake') + ' ' + peers.Statistics(peer.Identifier).LastHandshake"><i class="fa-solid fa-circle-info"></i></small>
            </div>
            <div v-else>
              <span class="badge rounded-pill bg-light" :title="$t('interfaces.peer-not-connected')"><i class="fa-solid fa-link-slash"></i></span>
            </div>
          </td>
          <td v-if="peers.hasStatistics" >
            <div class="d-flex flex-column">
              <span :title="humanFileSize(peers.Statistics(peer.Identifier).BytesReceived) + ' / ' + humanFileSize(peers.Statistics(peer.Identifier).BytesTransmitted)">
                <i class="fa-solid fa-arrow-down me-1"></i>{{ humanFileSize(peers.TrafficStats(peer.Identifier).Received) }}/s
                <i class="fa-solid fa-arrow-up ms-1 me-1"></i>{{ humanFileSize(peers.TrafficStats(peer.Identifier).Transmitted) }}/s
              </span>
            </div>
          </td>
          <td class="text-center">
            <a href="#" :title="$t('interfaces.button-show-peer')" @click.prevent="viewedPeerId=peer.Identifier"><i class="fas fa-eye me-2"></i></a>
            <a href="#" :title="$t('interfaces.button-edit-peer')" @click.prevent="editPeerId=peer.Identifier"><i class="fas fa-cog"></i></a>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <hr v-if="interfaces.Count!==0">
  <div v-if="interfaces.Count!==0" class="mt-3">
    <div class="row">
      <div class="col-6">
        <ul class="pagination pagination-sm">
          <li :class="{disabled:peers.pageOffset===0}" class="page-item">
            <a class="page-link" @click="peers.previousPage">&laquo;</a>
          </li>

          <li v-for="page in peers.pages" :key="page" :class="{active:peers.currentPage===page}" class="page-item">
            <a class="page-link" @click="peers.gotoPage(page)">{{page}}</a>
          </li>

          <li :class="{disabled:!peers.hasNextPage}" class="page-item">
            <a class="page-link" @click="peers.nextPage">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">{{ $t('general.pagination.size') }}:</label>
          <div class="col-sm-6">
            <select id="paginationSelector" v-model.number="peers.pageSize" class="form-select" @click="peers.afterPageSizeChange()">
              <option value="10">10</option>
              <option value="25">25</option>
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="999999999">{{ $t('general.pagination.all') }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<style>
</style>
