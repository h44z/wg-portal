import { defineStore } from 'pinia'

export const interfaceStore = defineStore({
  id: 'interfaces',
  state: () => ({
    interfaces: [],
    selected: "wg0",
    fetching: false,
  }),
  getters: {
    Count: (state) => state.interfaces.length,
    All: (state) => state.interfaces,
    GetSelected: (state) => state.interfaces.filter((i) => i.Identifier === state.selected)[0],
    isFetching: (state) => state.fetching,
  },
  actions: {
    async fetch() {
      this.fetching = true;
      /*const response = await fetch('/data/new-arrivals.json');
      try {
        const result = await response.json();
        this.interfaces = result.interfaces;
      } catch (err) {
        this.interfaces = [];
        console.error('Error loading interfaces:', err);
        return err;
      }*/
      this.interfaces = [{
        Identifier: "wg0",
        Mode:"server"
      },{
        Identifier: "wg1",
        Mode:"client"
      }];

      this.fetching = false;
    }
  }
})
