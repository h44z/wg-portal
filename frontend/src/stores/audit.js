import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import { base64_url_encode } from '@/helpers/encoding';

const baseUrl = `/audit`

export const auditStore = defineStore('audit', {
  state: () => ({
    entries: [],
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
  }),
  getters: {
    Count: (state) => state.entries.length,
    FilteredCount: (state) => state.Filtered.length,
    All: (state) => state.entries,
    Filtered: (state) => {
      if (!state.filter) {
        return state.entries
      }
      return state.entries.filter((e) => {
        return e.Timestamp.includes(state.filter) ||
            e.Message.includes(state.filter) ||
            e.Severity.includes(state.filter) ||
            e.Origin.includes(state.filter)
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
    setEntries(entries) {
      this.entries = entries
      this.calculatePages()
      this.fetching = false
    },
    async LoadEntries() {
      this.fetching = true
      return apiWrapper.get(`${baseUrl}/entries`)
        .then(this.setEntries)
        .catch(error => {
          this.setEntries([])
          console.log("Failed to load audit entries: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load audit entries!",
          })
        })
    },
  }
})
