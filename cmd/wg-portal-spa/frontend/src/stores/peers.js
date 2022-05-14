import { defineStore } from 'pinia'

export const peerStore = defineStore({
  id: 'peers',
  state: () => ({
    peers: [],
    filter: "",
    pageSize: 10,
    pageOffset: 0,
    pages: [],
    fetching: false,
  }),
  getters: {
    Count: (state) => state.peers.length,
    All: (state) => state.peers,
    Filtered: (state) => state.peers.slice(state.pageOffset, state.pageOffset + state.pageSize), // TODO: filter
    isFetching: (state) => state.fetching,
    hasNextPage: (state) => state.pageOffset < (state.peers.length - state.pageSize),
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
      for (let i = 0; i < this.peers.length; i+=this.pageSize) {
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
    async fetch() {
      this.fetching = true
      /*const response = await fetch('/data/new-arrivals.json');
      try {
        const result = await response.json();
        this.peers = result.peers;
      } catch (err) {
        this.peers = [];
        console.error('Error loading peers:', err);
        return err;
      }*/
      this.peers = [{
        Identifier: "id1",
        Name:"Testing name"
      },{
        Identifier: "id2",
        Name:"Testing name 2"
      },{
        Identifier: "id3",
        Name:"Testing name 2"
      },{
        Identifier: "id4",
        Name:"Testing name 2"
      },{
        Identifier: "id5",
        Name:"Testing name 2"
      },{
        Identifier: "id6",
        Name:"Testing name 2"
      },{
        Identifier: "id7",
        Name:"Testing name 2"
      },{
        Identifier: "id8",
        Name:"Testing name 2"
      },{
        Identifier: "id9",
        Name:"Testing name 2"
      },{
        Identifier: "id10",
        Name:"Testing name 2"
      },{
        Identifier: "id11",
        Name:"Testing name 2"
      },{
        Identifier: "id12",
        Name:"Testing name 2"
      },{
        Identifier: "id13",
        Name:"Testing name 2"
      },{
        Identifier: "id14",
        Name:"Testing name 2"
      },{
        Identifier: "id15",
        Name:"Testing name 2"
      },{
        Identifier: "id16",
        Name:"Testing name 2"
      },{
        Identifier: "id17",
        Name:"Testing name 2"
      },{
        Identifier: "id18",
        Name:"Testing name 2"
      },{
        Identifier: "id19",
        Name:"Testing name 2"
      },{
        Identifier: "id20",
        Name:"Testing name 2"
      },{
        Identifier: "id21",
        Name:"Testing name 2"
      },{
        Identifier: "id22",
        Name:"Testing name 2"
      },{
        Identifier: "id23",
        Name:"Testing name 2"
      },{
        Identifier: "id24",
        Name:"Testing name 2"
      },{
        Identifier: "id25",
        Name:"Testing name 2"
      },{
        Identifier: "id26",
        Name:"Testing name 2"
      },{
        Identifier: "id27",
        Name:"Testing name 2"
      }]

      this.fetching = false
      this.calculatePages()
    }
  }
})
