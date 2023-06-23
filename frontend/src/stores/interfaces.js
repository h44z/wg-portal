import { defineStore } from 'pinia'

import {apiWrapper} from '@/helpers/fetch-wrapper'
import {notify} from "@kyvg/vue3-notification";
import { freshInterface } from '@/helpers/models';

const baseUrl = `/interface`

export const interfaceStore = defineStore({
  id: 'interfaces',
  state: () => ({
    interfaces: [],
    prepared: freshInterface(),
    configuration: "",
    selected: "",
    fetching: false,
  }),
  getters: {
    Count: (state) => state.interfaces.length,
    Prepared: (state) => {console.log("STATE:", state.prepared); return state.prepared},
    All: (state) => state.interfaces,
    Find: (state) => {
        return (id) => state.interfaces.find((p) => p.Identifier === id)
    },
    GetSelected: (state) => state.interfaces.find((i) => i.Identifier === state.selected) || state.interfaces[0],
    isFetching: (state) => state.fetching,
  },
  actions: {
    setInterfaces(interfaces) {
      this.interfaces = interfaces
      if (this.interfaces.length > 0) {
        this.selected = this.interfaces[0].Identifier
      } else {
        this.selected = ""
      }
      this.fetching = false
    },
    async LoadInterfaces() {
      this.fetching = true
      return apiWrapper.get(`${baseUrl}/all`)
          .then(this.setInterfaces)
          .catch(error => {
            this.setInterfaces([])
            console.log("Failed to load interfaces: ", error)
            notify({
              title: "Backend Connection Failure",
              text: "Failed to load interfaces!",
            })
          })
    },
    setPreparedInterface(iface) {
      this.prepared = iface;
    },
    setInterfaceConfig(ifaceConfig) {
        this.configuration = ifaceConfig;
    },
    async PrepareInterface() {
      return apiWrapper.get(`${baseUrl}/prepare`)
        .then(this.setPreparedInterface)
        .catch(error => {
          this.prepared = freshInterface()
          console.log("Failed to load prepared interface: ", error)
          notify({
            title: "Backend Connection Failure",
            text: "Failed to load prepared interface!",
          })
        })
    },
    async InterfaceConfig(id) {
      return apiWrapper.get(`${baseUrl}/config/${encodeURIComponent(id)}`)
          .then(this.setInterfaceConfig)
          .catch(error => {
              this.prepared = {}
              console.log("Failed to load interface configuration: ", error)
              notify({
                  title: "Backend Connection Failure",
                  text: "Failed to load interface configuration!",
              })
          })
    },
    async DeleteInterface(id) {
      this.fetching = true
      return apiWrapper.delete(`${baseUrl}/${encodeURIComponent(id)}`)
        .then(() => {
          this.interfaces = this.interfaces.filter(i => i.Identifier !== id)
          if (this.interfaces.length > 0) {
            this.selected = this.interfaces[0].Identifier
          } else {
            this.selected = ""
          }
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async UpdateInterface(id, formData) {
      this.fetching = true
      return apiWrapper.put(`${baseUrl}/${encodeURIComponent(id)}`, formData)
        .then(iface => {
          let idx = this.interfaces.findIndex((i) => i.Identifier === id)
          this.interfaces[idx] = iface
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    },
    async CreateInterface(formData) {
      this.fetching = true
      return apiWrapper.post(`${baseUrl}/new`, formData)
        .then(iface => {
          this.interfaces.push(iface)
          this.fetching = false
        })
        .catch(error => {
          this.fetching = false
          console.log(error)
          throw new Error(error)
        })
    }
  }
})
