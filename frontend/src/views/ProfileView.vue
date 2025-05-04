<script setup>
import PeerViewModal from "../components/PeerViewModal.vue";

import { onMounted, ref } from "vue";
import { profileStore } from "@/stores/profile";
import UserPeerEditModal from "@/components/UserPeerEditModal.vue";
import { settingsStore } from "@/stores/settings";
import { humanFileSize } from "@/helpers/utils";

const settings = settingsStore()
const profile = profileStore()

const viewedPeerId = ref("")
const editPeerId = ref("")

const sortKey = ref("")
const sortOrder = ref(1)
const selectAll = ref(false)

function sortBy(key) {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value * -1; // Toggle sort order
  } else {
    sortKey.value = key;
    sortOrder.value = 1; // Default to ascending
  }
  profile.sortKey = sortKey.value;
  profile.sortOrder = sortOrder.value;
}

function friendlyInterfaceName(id, name) {
  if (name) {
    return name
  }
  return id
}

function toggleSelectAll() {
  profile.FilteredAndPagedPeers.forEach(peer => {
    peer.IsSelected = selectAll.value;
  });
}

onMounted(async () => {
  await profile.LoadUser()
  await profile.LoadPeers()
  await profile.LoadStats()
  await profile.LoadInterfaces()
  await profile.calculatePages(); // Forces to show initial page number
})

</script>

<template>
  <PeerViewModal :peerId="viewedPeerId" :visible="viewedPeerId !== ''" @close="viewedPeerId = ''"></PeerViewModal>
  <UserPeerEditModal :peerId="editPeerId" :visible="editPeerId !== ''" @close="editPeerId = ''; profile.LoadPeers()"></UserPeerEditModal>

  <!-- Peer list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-5">
      <h2 class="mt-2">{{ $t('profile.headline') }}</h2>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="profile.filter" class="form-control" :placeholder="$t('general.search.placeholder')" type="text"
            @keyup="profile.afterPageSizeChange">
          <button class="input-group-text btn btn-primary" :title="$t('general.search.button')"><i
              class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-3 text-lg-end">
      <div class="form-group" v-if="settings.Setting('SelfProvisioning')">
        <div class="input-group mb-3">
          <button class="input-group-text btn btn-primary" :title="$t('interfaces.button-add-peer')" @click.prevent="editPeerId = '#NEW#'">
            <i class="fa fa-plus me-1"></i><i class="fa fa-user"></i>
          </button>
          <select v-model="profile.selectedInterfaceId" :disabled="profile.CountInterfaces===0" class="form-select">
            <option v-if="profile.CountInterfaces===0" value="nothing">{{ $t('interfaces.no-interface.default-selection') }}</option>
            <option v-for="iface in profile.interfaces" :key="iface.Identifier" :value="iface.Identifier">{{ friendlyInterfaceName(iface.Identifier,iface.DisplayName) }}</option>
          </select>
        </div>
      </div>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <div v-if="profile.CountPeers === 0">
      <h4>{{ $t('profile.no-peer.headline') }}</h4>
      <p>{{ $t('profile.no-peer.abstract') }}</p>
    </div>
    <table v-if="profile.CountPeers !== 0" id="peerTable" class="table table-sm">
      <thead>
        <tr>
          <th scope="col">
            <input class="form-check-input" :title="$t('general.select-all')" type="checkbox" v-model="selectAll" @change="toggleSelectAll">
          </th><!-- select -->
          <th scope="col"></th><!-- status -->
          <th scope="col" @click="sortBy('DisplayName')">
            {{ $t("profile.table-heading.name") }}
            <i v-if="sortKey === 'DisplayName'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
          </th>
          <th scope="col" @click="sortBy('Addresses')">
            {{ $t("profile.table-heading.ip") }}
            <i v-if="sortKey === 'Addresses'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
          </th>
          <th v-if="profile.hasStatistics" scope="col" @click="sortBy('IsConnected')">
            {{ $t("profile.table-heading.stats") }}
            <i v-if="sortKey === 'IsConnected'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
          </th>
          <th v-if="profile.hasStatistics" scope="col" @click="sortBy('Traffic')">RX/TX
            <i v-if="sortKey === 'Traffic'" :class="sortOrder === 1 ? 'asc' : 'desc'"></i>
          </th>
          <th scope="col">{{ $t('profile.table-heading.interface') }}</th>
          <th scope="col"></th><!-- Actions -->
        </tr>
      </thead>
      <tbody>
        <tr v-for="peer in profile.FilteredAndPagedPeers" :key="peer.Identifier">
          <th scope="row">
            <input class="form-check-input" type="checkbox" v-model="peer.IsSelected">
          </th>
          <td class="text-center">
            <span v-if="peer.Disabled" class="text-danger"><i class="fa fa-circle-xmark"
                :title="peer.DisabledReason"></i></span>
            <span v-if="!peer.Disabled && peer.ExpiresAt" class="text-warning"><i class="fas fa-hourglass-end"
                :title="peer.ExpiresAt"></i></span>
          </td>
          <td><span v-if="peer.DisplayName" :title="peer.Identifier">{{ peer.DisplayName }}</span><span v-else
              :title="peer.Identifier">{{ $filters.truncate(peer.Identifier, 10) }}</span></td>
          <td>
            <span v-for="ip in peer.Addresses" :key="ip" class="badge rounded-pill bg-light">{{ ip }}</span>
          </td>
          <td v-if="profile.hasStatistics">
            <div v-if="profile.Statistics(peer.Identifier).IsConnected">
              <span class="badge rounded-pill bg-success"><i class="fa-solid fa-link"></i></span>
              <span :title="profile.Statistics(peer.Identifier).LastHandshake">{{ $t('profile.peer-connected') }}</span>
            </div>
            <div v-else>
              <span class="badge rounded-pill bg-light"><i class="fa-solid fa-link-slash"></i></span>
            </div>
          </td>
          <td v-if="profile.hasStatistics" >
            <span class="text-center" >{{ humanFileSize(profile.Statistics(peer.Identifier).BytesReceived) }} / {{ humanFileSize(profile.Statistics(peer.Identifier).BytesTransmitted) }}</span>
          </td>
          <td>{{ peer.InterfaceIdentifier }}</td>
          <td class="text-center">
            <a href="#" :title="$t('profile.button-show-peer')" @click.prevent="viewedPeerId = peer.Identifier"><i
                class="fas fa-eye me-2"></i></a>
            <a href="#" :title="$t('profile.button-edit-peer')" @click.prevent="editPeerId = peer.Identifier"><i
                class="fas fa-cog"></i></a>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <hr>
  <div class="mt-3">
    <div class="row">
      <div class="col-6">
        <ul class="pagination pagination-sm">
          <li :class="{ disabled: profile.pageOffset === 0 }" class="page-item">
            <a class="page-link" @click="profile.previousPage">&laquo;</a>
          </li>

          <li v-for="page in profile.pages" :key="page" :class="{ active: profile.currentPage === page }" class="page-item">
            <a class="page-link" @click="profile.gotoPage(page)">{{ page }}</a>
          </li>

          <li :class="{ disabled: !profile.hasNextPage }" class="page-item">
            <a class="page-link" @click="profile.nextPage">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">
            {{ $t('general.pagination.size')}}:
          </label>
          <div class="col-sm-6">
            <select id="paginationSelector" v-model.number="profile.pageSize" class="form-select" @click="profile.afterPageSizeChange()">
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
</div></template>
