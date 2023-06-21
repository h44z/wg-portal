import { defineStore } from 'pinia'
import {apiWrapper} from "../helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import {interfaceStore} from "./interfaces";

const baseUrl = `/peer`

export const peerStore = defineStore({
  id: 'peers',
  state: () => ({
    peers: [],
    prepared: {
      Identifier: "",
    },
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
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.FilteredCount - state.pageSize),
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
    setPreparedPeer(peer) {
      this.prepared = peer;
    },
    async PreparePeer(interfaceId) {
      return apiWrapper.get(`${baseUrl}/iface/${iface.Identifier}/prepare`)
        .then(this.setPreparedPeer)
        .catch(error => {
          this.prepared = {}
          console.log("Failed to load prepared peer: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load prepared peer!",
          })
        })
    },
    async LoadPeers() {
      let iface = interfaceStore().GetSelected
      if (!iface) {
        return // no interface, nothing to load
      }
      this.fetching = true

      return apiWrapper.get(`${baseUrl}/iface/${iface.Identifier}/all`)
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
