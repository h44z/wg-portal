import { defineStore } from 'pinia'
import {apiWrapper} from "@/helpers/fetch-wrapper";
import {notify} from "@kyvg/vue3-notification";
import { base64_url_encode } from '@/helpers/encoding';

const baseUrl = `/user`

export const userStore = defineStore('users', {
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
      return apiWrapper.delete(`${baseUrl}/${base64_url_encode(id)}`)
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
      return apiWrapper.put(`${baseUrl}/${base64_url_encode(id)}`, formData)
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
      return apiWrapper.get(`${baseUrl}/${base64_url_encode(id)}/peers`)
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
    async BulkDelete(ids) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-delete`, { Identifiers: ids })
        .then(() => {
          this.users = this.users.filter(u => !ids.includes(u.Identifier))
          this.fetching = false
          notify({
            title: "Users deleted",
            text: "Selected users have been deleted!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to delete users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to delete selected users!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkEnable(ids) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-enable`, { Identifiers: ids })
        .then(() => {
          this.users.forEach(u => {
            if (ids.includes(u.Identifier)) {
              u.Disabled = false
              u.DisabledReason = ""
            }
          })
          this.fetching = false
          notify({
            title: "Users enabled",
            text: "Selected users have been enabled!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to enable users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to enable selected users!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkDisable(ids, reason) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-disable`, { Identifiers: ids, Reason: reason })
        .then(() => {
          this.users.forEach(u => {
            if (ids.includes(u.Identifier)) {
              u.Disabled = true
              u.DisabledReason = reason
            }
          })
          this.fetching = false
          notify({
            title: "Users disabled",
            text: "Selected users have been disabled!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to disable users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to disable selected users!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkLock(ids, reason) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-lock`, { Identifiers: ids, Reason: reason })
        .then(() => {
          this.users.forEach(u => {
            if (ids.includes(u.Identifier)) {
              u.Locked = true
              u.LockedReason = reason
            }
          })
          this.fetching = false
          notify({
            title: "Users locked",
            text: "Selected users have been locked!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to lock users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to lock selected users!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
    async BulkUnlock(ids) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/bulk-unlock`, { Identifiers: ids })
        .then(() => {
          this.users.forEach(u => {
            if (ids.includes(u.Identifier)) {
              u.Locked = false
              u.LockedReason = ""
            }
          })
          this.fetching = false
          notify({
            title: "Users unlocked",
            text: "Selected users have been unlocked!",
            type: 'success',
          })
        })
        .catch(error => {
          this.fetching = false
          console.log("Failed to unlock users: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to unlock selected users!",
            type: 'error',
          })
          throw new Error(error)
        })
    },
  }
})
