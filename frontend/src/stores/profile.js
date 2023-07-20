import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {authStore} from "@/stores/auth";
import { base64_url_encode } from '@/helpers/encoding';
import {freshStats} from "@/helpers/models";

const baseUrl = `/user`

export const profileStore = defineStore({
  id: 'profile',
  state: () => ({
    peers: [],
    stats: {},
    statsEnabled: false,
    user: {},
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
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
    FilteredAndPagedPeers: (state) => {
      return state.FilteredPeers.slice(state.pageOffset, state.pageOffset + state.pageSize)
    },
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.FilteredPeerCount - state.pageSize),
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
  }
})
