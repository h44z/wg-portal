package wireguard

import (
	"net"
	"reflect"
	"testing"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/vishvananda/netlink"
)

func ignoreNetlinkError(addr *netlink.Addr, _ error) *netlink.Addr {
	return addr
}

func Test_broadcastAddr(t *testing.T) {
	tests := []struct {
		name string
		arg  *netlink.Addr
		want *netlink.Addr
	}{
		{
			name: "V4_0",
			arg:  ignoreNetlinkError(parseCIDR("10.0.0.0/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
		},
		{
			name: "V4_1",
			arg:  ignoreNetlinkError(parseCIDR("10.0.0.1/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
		},
		{
			name: "V4_2",
			arg:  ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
		},
		{
			name: "V6_0",
			arg:  ignoreNetlinkError(parseCIDR("fe80::/64")),
			want: ignoreNetlinkError(parseCIDR("fe80::ffff:ffff:ffff:ffff/64")),
		},
		{
			name: "V6_1",
			arg:  ignoreNetlinkError(parseCIDR("fe80::1:2:3/64")),
			want: ignoreNetlinkError(parseCIDR("fe80::ffff:ffff:ffff:ffff/64")),
		},
		{
			name: "V6_2",
			arg:  ignoreNetlinkError(parseCIDR("fe80::ffff:ffff:ffff:ffff/64")),
			want: ignoreNetlinkError(parseCIDR("fe80::ffff:ffff:ffff:ffff/64")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := broadcastAddr(tt.arg); got.String() != tt.want.String() {
				t.Errorf("broadcastAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_increaseIP(t *testing.T) {
	tests := []struct {
		name string
		ip   *netlink.Addr
		want *netlink.Addr
	}{
		{
			name: "V4_1",
			ip:   ignoreNetlinkError(parseCIDR("10.0.0.0/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.1/24")),
		},
		{
			name: "V4_2",
			ip:   ignoreNetlinkError(parseCIDR("10.0.0.2/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.3/24")),
		},
		{
			name: "V4_3",
			ip:   ignoreNetlinkError(parseCIDR("10.0.0.254/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
		},
		{
			name: "V4_4",
			ip:   ignoreNetlinkError(parseCIDR("10.0.0.255/24")),
			want: ignoreNetlinkError(parseCIDR("10.0.1.0/24")),
		},
		{
			name: "V4_5",
			ip:   ignoreNetlinkError(parseCIDR("10.0.0.5/32")),
			want: ignoreNetlinkError(parseCIDR("10.0.0.6/32")),
		},
		{
			name: "V6_1",
			ip:   ignoreNetlinkError(parseCIDR("2001:db8::/64")),
			want: ignoreNetlinkError(parseCIDR("2001:db8::1/64")),
		},
		{
			name: "V6_2",
			ip:   ignoreNetlinkError(parseCIDR("2001:db8::5/64")),
			want: ignoreNetlinkError(parseCIDR("2001:db8::6/64")),
		},
		{
			name: "V6_3",
			ip:   ignoreNetlinkError(parseCIDR("2001:0db8:0000:0000:ffff:ffff:ffff:fffe/64")),
			want: ignoreNetlinkError(parseCIDR("2001:0db8:0000:0000:ffff:ffff:ffff:ffff/64")),
		},
		{
			name: "V6_4",
			ip:   ignoreNetlinkError(parseCIDR("2001:0db8:0:0:ffff:ffff:ffff:ffff/64")),
			want: ignoreNetlinkError(parseCIDR("2001:db8:0:1::/64")),
		},
		{
			name: "V6_5",
			ip:   ignoreNetlinkError(parseCIDR("2001:0db8:0:0:ffff:ffff:ffff:ffff/128")),
			want: ignoreNetlinkError(parseCIDR("2001:0db8:0:1::/128")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			increaseIP(tt.ip)
			if !reflect.DeepEqual(tt.ip, tt.want) {
				t.Errorf("increaseIP() got = %v, want %v", tt.ip, tt.want)
			}
		})
	}
}

func Test_isV4(t *testing.T) {
	tests := []struct {
		name string
		arg  *netlink.Addr
		want bool
	}{
		{
			name: "V4",
			arg:  ignoreNetlinkError(parseCIDR("10.0.0.1/24")),
			want: true,
		},
		{
			name: "V4 network",
			arg:  ignoreNetlinkError(parseCIDR("10.0.0.0/24")),
			want: true,
		},
		{
			name: "V6",
			arg:  ignoreNetlinkError(parseCIDR("fe80::/64")),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isV4(tt.arg); got != tt.want {
				t.Errorf("isV4() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_wgCtrlManager_GetAllUsedIPs(t *testing.T) {
	type args struct {
		id persistence.InterfaceIdentifier
	}
	tests := []struct {
		name    string
		mgr     *wgCtrlManager
		args    args
		want    []*netlink.Addr
		wantErr bool
	}{
		{
			name:    "No Such Interface",
			mgr:     &wgCtrlManager{peers: make(map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig)},
			args:    args{id: "wg0"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "No Peers",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{"wg0": nil}},
			args:    args{id: "wg0"},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Wrong IP addresses",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("invalid", true)}},
					},
				},
			},
			args:    args{id: "wg0"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Single IP addresses",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.3/24", true)}},
					},
				},
			},
			args: args{id: "wg0"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("10.0.0.2/24")),
				ignoreNetlinkError(parseCIDR("10.0.0.3/24")),
			},
			wantErr: false,
		},
		{
			name: "Multiple IP addresses",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24,684D:1111:222:3333:4444:5555:6:77/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("1.1.1.1/30,10.0.0.3/24,8.8.8.8/32", true)}},
					},
				},
			},
			args: args{id: "wg0"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("1.1.1.1/30")),
				ignoreNetlinkError(parseCIDR("8.8.8.8/32")),
				ignoreNetlinkError(parseCIDR("10.0.0.2/24")),
				ignoreNetlinkError(parseCIDR("10.0.0.3/24")),
				ignoreNetlinkError(parseCIDR("684D:1111:222:3333:4444:5555:6:77/64")),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mgr.GetAllUsedIPs(tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllUsedIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAllUsedIPs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_wgCtrlManager_GetUsedIPs(t *testing.T) {
	type args struct {
		id         persistence.InterfaceIdentifier
		subnetCidr string
	}
	tests := []struct {
		name    string
		mgr     *wgCtrlManager
		args    args
		want    []*netlink.Addr
		wantErr bool
	}{
		{
			name:    "No Such Interface",
			mgr:     &wgCtrlManager{peers: make(map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig)},
			args:    args{id: "wg0", subnetCidr: "10.0.0.0/24"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "No Peers",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{"wg0": nil}},
			args:    args{id: "wg0", subnetCidr: "10.0.0.0/24"},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Wrong subnet",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{"wg0": nil}},
			args:    args{id: "wg0", subnetCidr: "subnet"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Wrong IP addresses",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("invalid", true)}},
					},
				},
			},
			args:    args{id: "wg0", subnetCidr: "10.0.0.0/24"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Single IP addresses V4",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.3/24", true)}},
					},
				},
			},
			args: args{id: "wg0", subnetCidr: "10.0.0.0/24"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("10.0.0.2/24")),
				ignoreNetlinkError(parseCIDR("10.0.0.3/24")),
			},
			wantErr: false,
		},
		{
			name: "Single IP addresses V6",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::5/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::6/64", true)}},
					},
				},
			},
			args: args{id: "wg0", subnetCidr: "2001:db8::/64"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("2001:db8::5/64")),
				ignoreNetlinkError(parseCIDR("2001:db8::6/64")),
			},
			wantErr: false,
		},
		{
			name: "Multiple IP addresses V4",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24,684D:1111:222:3333:4444:5555:6:77/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("1.1.1.1/30,10.0.0.3/24,8.8.8.8/32", true)}},
					},
				},
			},
			args: args{id: "wg0", subnetCidr: "10.0.0.0/24"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("10.0.0.2/24")),
				ignoreNetlinkError(parseCIDR("10.0.0.3/24")),
			},
			wantErr: false,
		},
		{
			name: "Multiple IP addresses V6",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24,2001:db8::5/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::6/64", true)}},
						"peer2": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db9::6/64,2001:db8:0:0:100::6/64", true)}},
					},
				},
			},
			args: args{id: "wg0", subnetCidr: "2001:db8::/64"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("2001:db8::5/64")),
				ignoreNetlinkError(parseCIDR("2001:db8::6/64")),
				ignoreNetlinkError(parseCIDR("2001:db8::100:0:0:6/64")),
			},
			wantErr: false,
		},
		{
			name: "Sort Order",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.3/16,10.0.0.2/16,10.0.5.2/16", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.1/16,10.0.4.2/16,10.0.6.2/16,10.0.5.3/16", true)}},
					},
				},
			},
			args: args{id: "wg0", subnetCidr: "10.0.0.0/16"},
			want: []*netlink.Addr{
				ignoreNetlinkError(parseCIDR("10.0.0.1/16")),
				ignoreNetlinkError(parseCIDR("10.0.0.2/16")),
				ignoreNetlinkError(parseCIDR("10.0.0.3/16")),
				ignoreNetlinkError(parseCIDR("10.0.4.2/16")),
				ignoreNetlinkError(parseCIDR("10.0.5.2/16")),
				ignoreNetlinkError(parseCIDR("10.0.5.3/16")),
				ignoreNetlinkError(parseCIDR("10.0.6.2/16")),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mgr.GetUsedIPs(tt.args.id, tt.args.subnetCidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUsedIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUsedIPs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_wgCtrlManager_GetFreshIp(t *testing.T) {
	type args struct {
		id         persistence.InterfaceIdentifier
		subnetCidr string
		increment  []bool
	}
	tests := []struct {
		name    string
		mgr     *wgCtrlManager
		args    args
		want    *netlink.Addr
		wantErr bool
	}{
		{
			name: "V4_1_noincrement",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.3/24", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "10.0.0.0/24",
			},
			want:    ignoreNetlinkError(parseCIDR("10.0.0.1/24")),
			wantErr: false,
		},
		{
			name: "V4_1_increment",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/24", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.3/24", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "10.0.0.0/24",
				increment:  []bool{true},
			},
			want:    ignoreNetlinkError(parseCIDR("10.0.0.4/24")),
			wantErr: false,
		},
		{
			name: "V4_1_overflow",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("10.0.0.2/32", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "10.0.0.2/32",
				increment:  []bool{true},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "V6_1_noincrement",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::5/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::6/64", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "2001:db8::/64",
			},
			want:    ignoreNetlinkError(parseCIDR("2001:db8::1/64")),
			wantErr: false,
		},
		{
			name: "V6_1_increment",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::5/64", true)}},
						"peer1": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::6/64", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "2001:db8::/64",
				increment:  []bool{true},
			},
			want:    ignoreNetlinkError(parseCIDR("2001:db8::7/64")),
			wantErr: false,
		},
		{
			name: "V6_1_overflow",
			mgr: &wgCtrlManager{
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Interface: &persistence.PeerInterfaceConfig{AddressStr: persistence.NewStringConfigOption("2001:db8::ffff/128", true)}},
					},
				},
			},
			args: args{
				id:         "wg0",
				subnetCidr: "2001:db8::/128",
				increment:  []bool{true},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mgr.GetFreshIp(tt.args.id, tt.args.subnetCidr, tt.args.increment...)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFreshIp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetFreshIp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		want    *netlink.Addr
		wantErr bool
	}{
		{
			name: "Valid V4",
			cidr: "10.0.0.1/24",
			want: &netlink.Addr{IPNet: &net.IPNet{
				IP:   net.IPv4(10, 0, 0, 1),
				Mask: net.IPv4Mask(255, 255, 255, 0)},
			},
			wantErr: false,
		},
		{
			name:    "Inalid V4",
			cidr:    "10.0.0.1/64",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid V6",
			cidr: "fe80::/128",
			want: &netlink.Addr{IPNet: &net.IPNet{
				IP:   []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				Mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
			},
			wantErr: false,
		},
		{
			name:    "Inalid V6",
			cidr:    "10:0:0::/256",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCIDR(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCIDR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCIDR() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseIpAddressString(t *testing.T) {
	type args struct {
		addrStr string
	}
	var tests = []struct {
		name    string
		args    args
		want    []*netlink.Addr
		wantErr bool
	}{
		{
			name:    "Empty String",
			args:    args{},
			want:    []*netlink.Addr{},
			wantErr: false,
		},
		{
			name:    "Single IPv4",
			args:    args{addrStr: "123.123.123.123"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Malformed",
			args:    args{addrStr: "hello world"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Single IPv4 CIDR",
			args: args{addrStr: "123.123.123.123/24"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(123, 123, 123, 123),
					Mask: net.IPv4Mask(255, 255, 255, 0),
				},
			}},
			wantErr: false,
		},
		{
			name: "Multiple IPv4 CIDR",
			args: args{addrStr: "123.123.123.123/24, 200.201.202.203/16"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(123, 123, 123, 123),
					Mask: net.IPv4Mask(255, 255, 255, 0),
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IPv4(200, 201, 202, 203),
					Mask: net.IPv4Mask(255, 255, 0, 0),
				},
			}},
			wantErr: false,
		},
		{
			name: "Single IPv6 CIDR",
			args: args{addrStr: "fe80::1/64"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
				},
			}},
			wantErr: false,
		},
		{
			name: "Multiple IPv6 CIDR",
			args: args{addrStr: "fe80::1/64 , 2130:d3ad::b33f/128"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IP{0x21, 0x30, 0xd3, 0xad, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xb3, 0x3f},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
				},
			}},
			wantErr: false,
		},
		{
			name: "Mixed IPv4 and IPv6 CIDR",
			args: args{addrStr: "200.201.202.203/16,2130:d3ad::b33f/128"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(200, 201, 202, 203),
					Mask: net.IPv4Mask(255, 255, 0, 0),
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IP{0x21, 0x30, 0xd3, 0xad, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xb3, 0x3f},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIpAddressString(tt.args.addrStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIpAddressString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIpAddressString() got = %v, want %v", got, tt.want)
			}
		})
	}
}
