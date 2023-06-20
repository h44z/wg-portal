<script setup>
import PeerViewModal from "../components/PeerViewModal.vue";

import {computed, onMounted, ref} from "vue";
import {profileStore} from "@/stores/profile";

const profile = profileStore()

const viewedPeerId = ref("")
const editPeerId = ref("")

onMounted(async () => {
  await profile.LoadUser()
  await profile.LoadPeers()
})
</script>

<template>
  <PeerViewModal :peerId="viewedPeerId" :visible="viewedPeerId!==''" @close="viewedPeerId=''"></PeerViewModal>

  <!-- Peer list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-5">
      <h2 class="mt-2">{{ $t('profile.h2-clients') }}</h2>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="profile.filter" class="form-control" placeholder="Search..." type="text" @keyup="profile.afterPageSizeChange">
          <button class="input-group-text btn btn-primary" title="Search"><i class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-3 text-lg-end">
      <a class="btn btn-primary ms-2" href="#" title="Add a peer" @click.prevent="editPeerId='#NEW#'"><i class="fa fa-plus me-1"></i><i class="fa fa-user"></i></a>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <div v-if="profile.CountPeers===0">
    <h4>{{ $t('profile.noPeerSelect.h4') }}</h4>
    <p>{{ $t('profile.noPeerSelect.message') }}</p>
    </div>
    <table v-if="profile.CountPeers!==0" id="peerTable" class="table table-sm">
      <thead>
      <tr>
        <th scope="col">
          <input id="flexCheckDefault" class="form-check-input" title="Select all" type="checkbox" value="">
        </th><!-- select -->
        <th scope="col">{{ $t('profile.tableHeadings[0]') }}</th>
        <th scope="col">{{ $t('profile.tableHeadings[1]') }}</th>
        <th scope="col">{{ $t('profile.tableHeadings[2]') }}</th>
        <th scope="col">{{ $t('profile.tableHeadings[3]') }}</th>
        <th scope="col">{{ $t('profile.tableHeadings[4]') }}</th>
        <th scope="col"></th><!-- Actions -->
      </tr>
      </thead>
      <tbody>
        <tr v-for="peer in profile.FilteredAndPagedPeers" :key="peer.Identifier">
          <th scope="row">
            <input id="flexCheckDefault" class="form-check-input" type="checkbox" value="">
          </th>
          <td>{{peer.DisplayName}}</td>
          <td>{{peer.Identifier}}</td>
          <td>{{peer.UserIdentifier}}</td>
          <td>
            <span v-for="ip in peer.Addresses" :key="ip" class="badge rounded-pill bg-light">{{ ip }}</span>
          </td>
          <td>{{peer.LastConnected}}</td>
          <td class="text-center">
            <a href="#" title="Show peer" @click.prevent="viewedPeerId=peer.Identifier"><i class="fas fa-eye me-2"></i></a>
            <a href="#" title="Edit peer" @click.prevent="editPeerId=peer.Identifier"><i class="fas fa-cog"></i></a>
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
          <li :class="{disabled:profile.pageOffset===0}" class="page-item">
            <a class="page-link" @click="profile.previousPage">&laquo;</a>
          </li>

          <li v-for="page in profile.pages" :key="page" :class="{active:profile.currentPage===page}" class="page-item">
            <a class="page-link" @click="profile.gotoPage(page)">{{page}}</a>
          </li>

          <li :class="{disabled:!profile.hasNextPage}" class="page-item">
            <a class="page-link" @click="profile.nextPage">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">{{ $t('profile.pagination.size') }}:</label>
          <div class="col-sm-6">
            <select v-model.number="profile.pageSize" class="form-select" @click="profile.afterPageSizeChange()">
              <option value="10">10</option>
              <option value="25">25</option>
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="999999999">{{ $t('profile.pagination.all') }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
