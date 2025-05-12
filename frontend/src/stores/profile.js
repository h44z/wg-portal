import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {authStore} from "@/stores/auth";
import { base64_url_encode } from '@/helpers/encoding';
import {freshStats} from "@/helpers/models";
import { ipToBigInt } from '@/helpers/utils';

const baseUrl = `/user`

export const profileStore = defineStore('profile', {
  state: () => ({
    peers: [],
    interfaces: [],
    selectedInterfaceId: "",
    stats: {},
    statsEnabled: false,
    user: {},
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
    sortKey: 'IsConnected', // Default sort key
    sortOrder: -1, // 1 for ascending, -1 for descending
  }),
  getters: {
    FindPeers: (state) => {
      return (id) => state.peers.find((p) => p.Identifier === id)
    },
    CountPeers: (state) => state.peers.length,
    FilteredPeerCount: (state) => state.FilteredPeers.length,
    Peers: (state) => state.peers,
    FilteredPeers: (state) => {
      if (!state.filter) {
        return state.peers
      }
      return state.peers.filter((p) => {
        return p.DisplayName.includes(state.filter) || p.Identifier.includes(state.filter)
      })
    },
    Sorted: (state) => {
      return state.FilteredPeers.slice().sort((a, b) => {
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
    FilteredAndPagedPeers: (state) => {
      return state.Sorted.slice(state.pageOffset, state.pageOffset + state.pageSize);
    },
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.FilteredPeerCount - state.pageSize),
    hasPrevPage: (state) => state.pageOffset > 0,
    currentPage: (state) => (state.pageOffset / state.pageSize)+1,
    Statistics: (state) => {
      return (id) => state.statsEnabled && (id in state.stats) ? state.stats[id] : freshStats()
    },
    hasStatistics: (state) => state.statsEnabled,
    CountInterfaces: (state) => state.interfaces.length,
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
      for (let i = 0; i < this.FilteredPeerCount; i+=this.pageSize) {
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
      this.fetching = false
    },
    setUser(user) {
      this.user = user
      this.fetching = false
    },
    setStats(statsResponse) {
      if (!statsResponse) {
        this.stats = {}
        this.statsEnabled = false
      }
      this.stats = statsResponse.Stats
      this.statsEnabled = statsResponse.Enabled
    },
    setInterfaces(interfaces) {
      this.interfaces = interfaces
      this.selectedInterfaceId = interfaces.length > 0 ? interfaces[0].Identifier : ""
      this.fetching = false
    },
    async enableApi() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.post(`${baseUrl}/${base64_url_encode(currentUser)}/api/enable`)
          .then(this.setUser)
          .catch(error => {
            this.fetching = false
            console.log("Failed to activate API for ", currentUser, ": ", error)
            notify({
              title: "Backend Connection Failure",
              text: "Failed to activate API!",
            })
          })
    },
    async disableApi() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.post(`${baseUrl}/${base64_url_encode(currentUser)}/api/disable`)
          .then(this.setUser)
          .catch(error => {
            this.fetching = false
            console.log("Failed to deactivate API for ", currentUser, ": ", error)
            notify({
              title: "Backend Connection Failure",
              text: "Failed to deactivate API!",
            })
          })
    },
    async LoadPeers() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(currentUser)}/peers`)
        .then(this.setPeers)
        .catch(error => {
          this.setPeers([])
          console.log("Failed to load user peers for ", currentUser, ": ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load user peers!",
          })
        })
    },
    async LoadStats() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(currentUser)}/stats`)
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
    async LoadUser() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(currentUser)}`)
        .then(this.setUser)
        .catch(error => {
          this.setUser({})
          console.log("Failed to load user for ", currentUser, ": ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load user!",
          })
        })
    },
    async LoadInterfaces() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(currentUser)}/interfaces`)
          .then(this.setInterfaces)
          .catch(error => {
            this.setInterfaces([])
            console.log("Failed to load interfaces for ", currentUser, ": ", error)
            notify({
              title: "Backend Connection Failure",
              text: "Failed to load interfaces!",
            })
          })
    },
  }
})
