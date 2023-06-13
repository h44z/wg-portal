<script setup>
import PeerViewModal from "../components/PeerViewModal.vue";
import PeerEditModal from "../components/PeerEditModal.vue";
import InterfaceEditModal from "../components/InterfaceEditModal.vue";

import {onMounted, ref} from "vue";
import {peerStore} from "../stores/peers";
import {interfaceStore} from "../stores/interfaces";

const interfaces = interfaceStore()
const peers = peerStore()

const viewedPeerId = ref("")
const editPeerId = ref("")
const editInterfaceId = ref("")

onMounted(async () => {
  await interfaces.LoadInterfaces()
  await peers.LoadPeers()
})
</script>

<template>
  <PeerViewModal :peerId="viewedPeerId" :visible="viewedPeerId!==''" @close="viewedPeerId=''"></PeerViewModal>
  <PeerEditModal :peerId="editPeerId" :visible="editPeerId!==''" @close="editPeerId=''"></PeerEditModal>
  <InterfaceEditModal :interfaceId="editInterfaceId" :visible="editInterfaceId!==''" @close="editInterfaceId=''"></InterfaceEditModal>

  <!-- Headline and interface selector -->
  <div class="page-header row">
    <div class="col-12 col-lg-8">
      <h1>{{ $t('interfaces.h1') }}</h1>
    </div>
    <div class="col-12 col-lg-4 text-end">
      <div class="form-group">

      </div>
      <div class="form-group">
        <div class="input-group mb-3">
          <button class="input-group-text btn btn-primary" title="Add new interface" @click.prevent="editInterfaceId='#NEW#'">
            <i class="fa-solid fa-plus-circle"></i>
          </button>
          <select v-model="interfaces.selected" :disabled="interfaces.Count===0" class="form-select" @change="peers.LoadPeers()">
            <option v-if="interfaces.Count===0" value="nothing">{{ $t('interfaces.notAvailable') }}</option>
            <option v-for="iface in interfaces.All" :key="iface.Identifier" :value="iface.Identifier">{{iface.Identifier}}</option>
          </select>
        </div>
      </div>
    </div>
  </div>

  <!-- No interfaces information -->
  <div v-if="interfaces.Count===0" class="row">
    <div class="col-lg-12">
      <div class="mt-5">
        <h4>{{ $t('interfaces.noInterface.h1') }}</h4>
        <p>{{ $t('interfaces.noInterface.message') }}</p>
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
              {{ $t('interfaces.statusBox.h1') }} <strong>{{interfaces.GetSelected.Identifier}}</strong> ({{interfaces.GetSelected.Mode}} {{ $t('interfaces.statusBox.mode') }})
            </div>
            <div class="col-12 col-lg-4 text-lg-end">
              <a class="btn-link" href="#" title="Show interface configuration"><i class="fas fa-eye"></i></a>
              <a class="ms-5 btn-link" href="#" title="Download interface configuration"><i class="fas fa-download"></i></a>
              <a class="ms-5 btn-link" href="#" title="Write interface configuration file"><i class="fas fa-save"></i></a>
              <a class="ms-5 btn-link" href="#" title="Edit interface settings" @click.prevent="editInterfaceId=interfaces.GetSelected.Identifier"><i class="fas fa-cog"></i></a>
            </div>
          </div>
        </div>
        <div class="card-body d-flex flex-column">
          <div v-if="interfaces.GetSelected.Mode==='server'" class="row">
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.statusBox.key') }}:</td>
                  <td>{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.endpoint') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefEndpoint}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.port') }}:</td>
                  <td>{{interfaces.GetSelected.ListenPort}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.peers') }}:</td>
                  <td>{{interfaces.GetSelected.EnabledPeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.totalPeers') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.statusBox.ip') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Addresses" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.dnsServers') }}:</td>
                  <td><span class="badge bg-light me-1" v-for="addr in interfaces.GetSelected.Dns" :key="addr">{{addr}}</span></td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.mtu') }}:</td>
                  <td>{{interfaces.GetSelected.Mtu}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.intervall') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefPersistentKeepalive}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.allowedIP') }}:</td>
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
                  <td>{{ $t('interfaces.statusBox.key') }}:</td>
                  <td>{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.endpoint') }}:</td>
                  <td>{{interfaces.GetSelected.InterfacePeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.totalPeers') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.statusBox.ip') }}:</td>
                  <td>{{interfaces.GetSelected.AddressStr}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.dnsServers') }}:</td>
                  <td>{{interfaces.GetSelected.DnsStr}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.mtu') }}:</td>
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
                  <td>{{ $t('interfaces.statusBox.key') }}:</td>
                  <td>{{interfaces.GetSelected.PublicKey}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.endpoint') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefEndpoint}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.port') }}:</td>
                  <td>{{interfaces.GetSelected.ListenPort}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.peers') }}:</td>
                  <td>{{interfaces.GetSelected.EnabledPeers}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.totalPeers') }}:</td>
                  <td>{{interfaces.GetSelected.TotalPeers}}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('interfaces.statusBox.ip') }}:</td>
                  <td>{{interfaces.GetSelected.Addresses}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.allowedIP') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefAllowedIPsStr}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.dnsServers') }}:</td>
                  <td>{{interfaces.GetSelected.PeerDefDnsStr}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.mtu') }}:</td>
                  <td>{{interfaces.GetSelected.Mtu}}</td>
                </tr>
                <tr>
                  <td>{{ $t('interfaces.statusBox.intervall') }}:</td>
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
      <h2 v-if="interfaces.GetSelected.Mode==='server'" class="mt-2">{{ $t('interfaces.h2') }}</h2>
      <h2 v-else class="mt-2">{{ $t('interfaces.h2-client') }}</h2>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="peers.filter" class="form-control" placeholder="Search..." type="text" @keyup="peers.afterPageSizeChange">
          <button class="input-group-text btn btn-primary" title="Search"><i class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-3 text-lg-end">
      <a v-if="interfaces.GetSelected.Mode==='server' && peers.Count!==0" class="btn btn-primary" href="#" title="Send mail to all peers"><i class="fa fa-paper-plane"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add multiple peers"><i class="fa fa-plus me-1"></i><i class="fa fa-users"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add a peer" @click.prevent="editPeerId='#NEW#'"><i class="fa fa-plus me-1"></i><i class="fa fa-user"></i></a>
    </div>
  </div>
  <div v-if="interfaces.Count!==0" class="mt-2 table-responsive">
    <div v-if="peers.Count===0">
    <h4>{{ $t('interfaces.noPeerSelect.h4') }}</h4>
    <p>{{ $t('interfaces.noPeerSelect.message') }}</p>
    </div>
    <table v-if="peers.Count!==0" id="peerTable" class="table table-sm">
      <thead>
      <tr>
        <th scope="col">
          <input id="flexCheckDefault" class="form-check-input" title="Select all" type="checkbox" value="">
        </th><!-- select -->
        <th scope="col">{{ $t('interfaces.tableHeadings[0]') }}</th>
        <th scope="col">{{ $t('interfaces.tableHeadings[1]') }}</th>
        <th scope="col">{{ $t('interfaces.tableHeadings[2]') }}</th>
        <th scope="col">{{ $t('interfaces.tableHeadings[3]') }}</th>
        <th v-if="interfaces.GetSelected.Mode==='client'" scope="col">{{ $t('interfaces.tableHeadings[4]') }}</th>
        <th scope="col">{{ $t('interfaces.tableHeadings[5]') }}</th>
        <th scope="col"></th><!-- Actions -->
      </tr>
      </thead>
      <tbody>
        <tr v-for="peer in peers.FilteredAndPaged" :key="peer.Identifier">
          <th scope="row">
            <input id="flexCheckDefault" class="form-check-input" type="checkbox" value="">
          </th>
          <td>{{peer.DisplayName}}</td>
          <td>{{peer.Identifier}}</td>
          <td>{{peer.UserIdentifier}}</td>
          <td>
            <span v-for="ip in peer.Addresses" :key="ip" class="badge rounded-pill bg-light">{{ ip }}</span>
          </td>
          <td v-if="interfaces.GetSelected.Mode==='client'">{{peer.Endpoint}}</td>
          <td>{{peer.LastConnected}}</td>
          <td class="text-center">
            <a href="#" title="Show peer" @click.prevent="viewedPeerId=peer.Identifier"><i class="fas fa-eye me-2"></i></a>
            <a href="#" title="Edit peer" @click.prevent="editPeerId=peer.Identifier"><i class="fas fa-cog"></i></a>
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
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">{{ $t('interfaces.pagination.size') }}:</label>
          <div class="col-sm-6">
            <select v-model.number="peers.pageSize" class="form-select" @click="peers.afterPageSizeChange()">
              <option value="10">10</option>
              <option value="25">25</option>
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="999999999">{{ $t('interfaces.pagination.all') }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
