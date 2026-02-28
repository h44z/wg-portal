import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {interfaceStore} from "./interfaces";
import {freshPeer, freshStats} from '@/helpers/models';
import { base64_url_encode } from '@/helpers/encoding';
import { ipToBigInt } from '@/helpers/utils';

const baseUrl = `/peer`

export const peerStore = defineStore('peers', {
  state: () => ({
    peers: [],
    stats: {},
    statsEnabled: false,
    peer: freshPeer(),
    prepared: freshPeer(),
    configuration: "",
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
    sortKey: 'IsConnected', // Default sort key
    sortOrder: -1, // 1 for ascending, -1 for descending
    trafficStats: {},
  }),
  getters: {
    Find: (state) => {
      return (id) => state.peers.find((p) => p.Identifier === id)
    },

    Count: (state) => state.peers.length,
    Prepared: (state) => {console.log("STATE:", state.prepared); return state.prepared},
    FilteredCount: (state) => state.Filtered.length,
    All: (state) => state.peers,
    Filtered: (state) => {
      if (!state.filter) {
        return state.peers
      }
      return state.peers.filter((p) => {
        return p.DisplayName.includes(state.filter) || p.Identifier.includes(state.filter)
      })
    },
    Sorted: (state) => {
      return state.Filtered.slice().sort((a, b) => {
        let aValue = a[state.sortKey];
        let bValue = b[state.sortKey];
        if (state.sortKey === 'Addresses') {
          aValue = aValue.length > 0 ? ipToBigInt(aValue[0]) : 0;
          bValue = bValue.length > 0 ? ipToBigInt(bValue[0]) : 0;
        }
        if (state.sortKey === 'IsConnected') {
          aValue = state.statsEnabled && state.stats[a.Identifier]?.IsConnected ? 1 : 0;
          bValue = state.statsEnabled && state.stats[b.Identifier]?.IsConnected ? 1 : 0;
        }
        if (state.sortKey === 'Traffic') {
          aValue = state.statsEnabled ? (state.stats[a.Identifier].BytesReceived + state.stats[a.Identifier].BytesTransmitted) : 0;
          bValue = state.statsEnabled ? (state.stats[b.Identifier].BytesReceived + state.stats[b.Identifier].BytesTransmitted) : 0;
        }
        let result = 0;
        if (aValue > bValue) result = 1;
        if (aValue < bValue) result = -1;
        return state.sortOrder === 1 ? result : -result;
      });
    },
    FilteredAndPaged: (state) => {
      return state.Sorted.slice(state.pageOffset, state.pageOffset + state.pageSize);
    },
    ConfigQrUrl: (state) => {
      return (id) => state.peers.find((p) => p.Identifier === id) ? apiWrapper.url(`${baseUrl}/config-qr/${base64_url_encode(id)}`) : ''
    },
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.FilteredCount - state.pageSize),
    hasPrevPage: (state) => state.pageOffset > 0,
    currentPage: (state) => (state.pageOffset / state.pageSize)+1,
    Statistics: (state) => {
      return (id) => state.statsEnabled && (id in state.stats) ? state.stats[id] : freshStats()
    },
    TrafficStats: (state) => {
      return (id) => (id in state.trafficStats) ? state.trafficStats[id] : { Received: 0, Transmitted: 0 }
    },
    hasStatistics: (state) => state.statsEnabled,

  },
  actions: {
    afterPageSizeChange() {
      // reset pageOffset to avoid problems with new page sizes
      this.pageOffset = 0
      this.calculatePages()
    },
    calculatePages() {
      let pageCounter = 1;
      this.pages = []
      for (let i = 0; i < this.FilteredCount; i+=this.pageSize) {
        this.pages.push(pageCounter++)
      }
    },
    gotoPage(page) {
      this.pageOffset = (page-1) * this.pageSize

      this.calculatePages()
    },
    nextPage() {
      this.pageOffset += this.pageSize

      this.calculatePages()
    },
    previousPage() {
      this.pageOffset -= this.pageSize

      this.calculatePages()
    },
    setPeers(peers) {
      this.peers = peers
      this.calculatePages()
      this.fetching = false
      this.trafficStats = {}
    },
    setPeer(peer) {
      this.peer = peer
      this.fetching = false
    },
    setPreparedPeer(peer) {
      this.prepared = peer;
    },
    setPeerConfig(config) {
      this.configuration = config;
    },
    setStats(statsResponse) {
      if (!statsResponse) {
        this.stats = {}
        this.statsEnabled = false
        this.trafficStats = {}
      } else {
          this.stats = statsResponse.Stats
          this.statsEnabled = statsResponse.Enabled
      }
    },
    updatePeerTrafficStats(peerStats) {
      const id = peerStats.EntityId;
      this.trafficStats[id] = {
        Received: peerStats.BytesReceived,
        Transmitted: peerStats.BytesTransmitted,
      };
    },
    async Reset() {
      this.setPeers([])
      this.setStats(undefined)
    },
    async PreparePeer(interfaceId) {
      return apiWrapper.get(`${baseUrl}/iface/${base64_url_encode(interfaceId)}/prepare`)
        .then(this.setPreparedPeer)
        .catch(error => {
          this.prepared = freshPeer()
          console.log("Failed to load prepared peer: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load prepared peer!",
          })
        })
    },
    async MailPeerConfig(linkOnly, style, ids) {
      return apiWrapper.post(`${baseUrl}/config-mail?style=${style}`, {
          Identifiers: ids,
          LinkOnly: linkOnly
        })
        .then(() => {
          notify({
            title: "Peer Configuration sent",
            text: "Email sent to linked user!",
          })
        })
        .catch(error => {
          console.log("Failed to send peer configuration: ", error)
          throw new Error(error)
        })
    },
    async LoadPeerConfig(id, style) {
      return apiWrapper.get(`${baseUrl}/config/${base64_url_encode(id)}?style=${style}`)
        .then(this.setPeerConfig)
        .catch(error => {
          this.configuration = ""
          console.log("Failed to load peer configuration: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load peer configuration!",
          })
        })
    },
    async LoadPeer(id) {
      this.fetching = true
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(id)}`)
        .then(this.setPeer)
        .catch(error => {
          this.setPeers([])
          console.log("Failed to load peer: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load peer!",
          })
        })
    },
    async LoadStats(interfaceId) {
      // if no interfaceId is given, use the currently selected interface
      if (!interfaceId) {
        if (!interfaceStore().GetSelected || !interfaceStore().GetSelected.Identifier) {
            return // no interface, nothing to load
        }
        interfaceId = interfaceStore().GetSelected.Identifier
      }
      this.fetching = true

      return apiWrapper.get(`${baseUrl}/iface/${base64_url_encode(interfaceId)}/stats`)
        .then(this.setStats)
        .catch(error => {
          this.setStats(undefined)
          console.log("Failed to load peer stats: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load peer stats!",
          })
        })
    },
    async DeletePeer(id) {
      this.fetching = true
      return apiWrapper.delete(`${baseUrl}/${base64_url_encode(id)}`)
        .then(() => {
          this.peers = this.peers.filter(p => p.Identifier !== id)
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async BulkDelete(ids) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-delete`, { Identifiers: ids })
        .then(() => {
          this.peers = this.peers.filter(p => !ids.includes(p.Identifier))
          this.fetching = false
          notify({
            title: "Peers deleted",
            text: "Selected peers have been deleted!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to delete peers: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to delete selected peers!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkEnable(ids) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-enable`, { Identifiers: ids })
        .then(async () => {
          await this.LoadPeers()
          notify({
            title: "Peers enabled",
            text: "Selected peers have been enabled!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to enable peers: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to enable selected peers!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkDisable(ids, reason) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-disable`, { Identifiers: ids, Reason: reason })
        .then(async () => {
          await this.LoadPeers()
          notify({
            title: "Peers disabled",
            text: "Selected peers have been disabled!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to disable peers: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to disable selected peers!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async UpdatePeer(id, formData) {
      this.fetching = true
      return apiWrapper.put(`${baseUrl}/${base64_url_encode(id)}`, formData)
        .then(peer => {
          let idx = this.peers.findIndex((p) => p.Identifier === id)
          this.peers[idx] = peer
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async CreatePeer(interfaceId, formData) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/iface/${base64_url_encode(interfaceId)}/new`, formData)
        .then(peer => {
          this.peers.push(peer)
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async CreateMultiplePeers(interfaceId, formData) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/iface/${base64_url_encode(interfaceId)}/multiplenew`, formData)
          .then(peers => {
            this.peers.push(...peers)
            this.fetching = false
          })
          .catch(error => {
            this.fetching = false
            console.log(error)
            throw new Error(error)
          })
    },
    async LoadPeers(interfaceId) {
      // if no interfaceId is given, use the currently selected interface
      if (!interfaceId) {
        if (!interfaceStore().GetSelected || !interfaceStore().GetSelected.Identifier) {
          return // no interface, nothing to load
        }
        interfaceId = interfaceStore().GetSelected.Identifier
      }
      this.fetching = true

      return apiWrapper.get(`${baseUrl}/iface/${base64_url_encode(interfaceId)}/all`)
        .then(this.setPeers)
        .catch(error => {
          this.setPeers([])
          console.log("Failed to load peers: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load peers!",
          })
        })
    }
  }
})
