import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {authStore} from "@/stores/auth";


const baseUrl = `/user`

export const profileStore = defineStore({
  id: 'profile',
  state: () => ({
    peers: [],
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
    async LoadPeers() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${encodeURIComponent(currentUser)}/peers`)
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
    async LoadUser() {
      this.fetching = true
      let currentUser = authStore().user.Identifier
      return apiWrapper.get(`${baseUrl}/${encodeURIComponent(currentUser)}`)
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
