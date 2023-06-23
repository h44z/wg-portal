
export function freshInterface() {
  return {
    Disabled: false,
    DisplayName: "",
    Identifier: "",
    Mode: "server",

    PublicKey: "",
    PrivateKey: "",

    ListenPort:  51820,
    Addresses: [],
    DnsStr: [],
    DnsSearch: [],

    Mtu: 0,
    FirewallMark: 0,
    RoutingTable: "",

    PreUp: "",
    PostUp: "",
    PreDown: "",
    PostDown: "",

    SaveConfig: false,

    // Peer defaults

    PeerDefNetwork: [],
    PeerDefDns: [],
    PeerDefDnsSearch: [],
    PeerDefEndpoint: "",
    PeerDefAllowedIPs: [],
    PeerDefMtu: 0,
    PeerDefPersistentKeepalive: 0,
    PeerDefFirewallMark: 0,
    PeerDefRoutingTable: "",
    PeerDefPreUp: "",
    PeerDefPostUp: "",
    PeerDefPreDown: "",
    PeerDefPostDown: ""
  }
}

export function freshPeer() {
  return {
    Identifier: "",
    DisplayName: "",
    UserIdentifier: "",
    InterfaceIdentifier: "",
    Disabled: false,
    ExpiresAt: null,
    Notes: "",

    Endpoint: {
      Value: "",
      Overridable: true,
    },
    EndpointPublicKey: {
      Value: "",
      Overridable: true,
    },
    AllowedIPs: {
      Value: [],
      Overridable: true,
    },
    ExtraAllowedIPs: [],
    PresharedKey: "",
    PersistentKeepalive: {
      Value: 0,
      Overridable: true,
    },

    PrivateKey: "",
    PublicKey: "",

    Mode: "client",

    Addresses: [],
    CheckAliveAddress: "",
    Dns: {
      Value: [],
      Overridable: true,
    },
    DnsSearch: {
      Value: [],
      Overridable: true,
    },
    Mtu: {
      Value: 0,
      Overridable: true,
    },
    FirewallMark: {
      Value: 0,
      Overridable: true,
    },
    RoutingTable: {
      Value: "",
      Overridable: true,
    },

    PreUp: {
      Value: "",
      Overridable: true,
    },
    PostUp: {
      Value: "",
      Overridable: true,
    },
    PreDown: {
      Value: "",
      Overridable: true,
    },
    PostDown: {
      Value: "",
      Overridable: true,
    }
  }
}