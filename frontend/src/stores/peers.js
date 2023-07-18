import { defineStore } from 'pinia'
import {apiWrapper} from "../helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {interfaceStore} from "./interfaces";
import {freshPeer, freshStats} from '@/helpers/models';
import { base64_url_encode } from '@/helpers/encoding';

const baseUrl = `/peer`

export const peerStore = defineStore({
  id: 'peers',
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
    FilteredAndPaged: (state) => {
      return state.Filtered.slice(state.pageOffset, state.pageOffset + state.pageSize)
    },
    ConfigQrUrl: (state) => {
      return (id) => apiWrapper.url(`${baseUrl}/config-qr/${base64_url_encode(id)}`)
    },
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.FilteredCount - state.pageSize),
    hasPrevPage: (state) => state.pageOffset > 0,
    currentPage: (state) => (state.pageOffset / state.pageSize)+1,
    Statistics: (state) => {
      return (id) => state.statsEnabled && (id in state.stats) ? state.stats[id] : freshStats()
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
      }
      this.stats = statsResponse.Stats
      this.statsEnabled = statsResponse.Enabled
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
    async MailPeerConfig(linkOnly, ids) {
      return apiWrapper.post(`${baseUrl}/config-mail`, {
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
    async LoadPeerConfig(id) {
      return apiWrapper.get(`${baseUrl}/config/${base64_url_encode(id)}`)
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
        interfaceId = interfaceStore().GetSelected.Identifier
        if (!interfaceId) {
          return // no interface, nothing to load
        }
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
        interfaceId = interfaceStore().GetSelected.Identifier
        if (!interfaceId) {
          return // no interface, nothing to load
        }
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
