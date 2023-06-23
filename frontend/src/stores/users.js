import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";

const baseUrl = `/user`

export const userStore = defineStore({
  id: 'users',
  state: () => ({
    userPeers: [],
    users: [],
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
  }),
  getters: {
    Find: (state) => {
      return (id) => state.users.find((p) => p.Identifier === id)
    },
    Count: (state) => state.users.length,
    FilteredCount: (state) => state.Filtered.length,
    All: (state) => state.users,
    Peers: (state) => state.userPeers,
    Filtered: (state) => {
      if (!state.filter) {
        return state.users
      }
      return state.users.filter((u) => {
        return u.Firstname.includes(state.filter) || u.Lastname.includes(state.filter) || u.Email.includes(state.filter) || u.Identifier.includes(state.filter)
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
    setUsers(users) {
      this.users = users
      this.calculatePages()
      this.fetching = false
    },
    setUserPeers(peers) {
      this.userPeers = peers
      this.fetching = false
    },
    async LoadUsers() {
      this.fetching = true
      return apiWrapper.get(`${baseUrl}/all`)
        .then(this.setUsers)
        .catch(error => {
          this.setUsers([])
          console.log("Failed to load users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load users!",
          })
        })
    },
    async DeleteUser(id) {
      this.fetching = true
      return apiWrapper.delete(`${baseUrl}/${encodeURIComponent(id)}`)
        .then(() => {
          this.users = this.users.filter(u => u.Identifier !== id)
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async UpdateUser(id, formData) {
      this.fetching = true
      return apiWrapper.put(`${baseUrl}/${encodeURIComponent(id)}`, formData)
        .then(user => {
          let idx = this.users.findIndex((u) => u.Identifier === id)
          this.users[idx] = user
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async CreateUser(formData) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/new`, formData)
        .then(user => {
          this.users.push(user)
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async LoadUserPeers(id) {
      this.fetching = true
      return apiWrapper.get(`${baseUrl}/${encodeURIComponent(id)}/peers`)
        .then(this.setUserPeers)
        .catch(error => {
          this.setUserPeers([])
          console.log("Failed to load user peers for ",id ,": ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load user peers!",
          })
        })
    },
  }
})
