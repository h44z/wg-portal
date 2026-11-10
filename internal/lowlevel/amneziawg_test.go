package lowlevel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/domain"
)

const awgTestPrivateKey = "gI6EdUSYvn8ugXOt8QQD6Yc+JyiZxIhp3GInSWRfWGE="
const awgTestPublicKey = "HIgo9xNzJMWLKASShiTqIybxZ0U3wGLiUeJ1PKf8ykw="

func awgKernelLink(name string) (netlink.Link, error) {
	return &netlink.GenericLink{Name: name, LinkType: "amneziawg"}, nil
}

func awgTestReply(version uint8, attrs ...*nl.RtAttr) []byte {
	data := []byte{0, version, 0, 0}
	for _, attr := range attrs {
		data = append(data, attr.Serialize()...)
	}
	return data
}

func awgTestAttrMap(t *testing.T, data []byte) map[uint16][]byte {
	t.Helper()
	attrs, err := nl.ParseRouteAttr(data)
	require.NoError(t, err)
	result := map[uint16][]byte{}
	for _, attr := range attrs {
		result[attr.Attr.Type&0x3fff] = attr.Value
	}
	return result
}

func awgTestSerialize(attrs []*nl.RtAttr) []byte {
	var data []byte
	for _, attr := range attrs {
		data = append(data, attr.Serialize()...)
	}
	return data
}

func awgTestIPs(address string, prefix byte) *nl.RtAttr {
	allowed := nl.NewRtAttr(unix.WGPEER_A_ALLOWEDIPS|unix.NLA_F_NESTED, nil)
	entry := allowed.AddRtAttr(unix.NLA_F_NESTED, nil)
	ip, family := net.ParseIP(address).To4(), uint16(unix.AF_INET)
	if ip == nil {
		ip, family = net.ParseIP(address).To16(), unix.AF_INET6
	}
	entry.AddRtAttr(unix.WGALLOWEDIP_A_FAMILY, binary.NativeEndian.AppendUint16(nil, family))
	entry.AddRtAttr(unix.WGALLOWEDIP_A_IPADDR, ip)
	entry.AddRtAttr(unix.WGALLOWEDIP_A_CIDR_MASK, []byte{prefix})
	return allowed
}

func awgTestDeviceReplies(t *testing.T, version uint8) [][]byte {
	t.Helper()
	private, err := wgtypes.ParseKey(awgTestPrivateKey)
	require.NoError(t, err)
	public, err := wgtypes.ParseKey(awgTestPublicKey)
	require.NoError(t, err)
	peers := nl.NewRtAttr(unix.WGDEVICE_A_PEERS|unix.NLA_F_NESTED, nil)
	peer := peers.AddRtAttr(unix.NLA_F_NESTED, nil)
	peer.AddRtAttr(unix.WGPEER_A_PUBLIC_KEY, public[:])
	peer.AddRtAttr(unix.WGPEER_A_RX_BYTES, binary.NativeEndian.AppendUint64(nil, 123))
	peer.AddRtAttr(unix.WGPEER_A_TX_BYTES, binary.NativeEndian.AppendUint64(nil, 456))
	peer.AddRtAttr(unix.WGPEER_A_LAST_HANDSHAKE_TIME, append(binary.NativeEndian.AppendUint64(nil, 1700000000), binary.NativeEndian.AppendUint64(nil, 42)...))
	keepalive := binary.NativeEndian.AppendUint16(nil, 25)
	if version >= 3 {
		keepalive = binary.NativeEndian.AppendUint32(nil, 35<<16|25)
	}
	peer.AddRtAttr(unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL, keepalive)
	endpoint := make([]byte, 28)
	binary.NativeEndian.PutUint16(endpoint, unix.AF_INET6)
	binary.BigEndian.PutUint16(endpoint[2:], 1234)
	copy(endpoint[8:], net.ParseIP("2001:db8::1").To16())
	peer.AddRtAttr(unix.WGPEER_A_ENDPOINT, endpoint)
	peer.AddChild(awgTestIPs("10.0.0.2", 32))
	header := binary.NativeEndian.AppendUint32(nil, 100)
	if version == 2 {
		header = []byte("100-200\x00")
	}
	if version == 3 {
		header = binary.NativeEndian.AppendUint64(nil, 200<<32|100)
	}
	first := awgTestReply(version,
		nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, []byte("awg0\x00")),
		nl.NewRtAttr(unix.WGDEVICE_A_PRIVATE_KEY, private[:]),
		nl.NewRtAttr(unix.WGDEVICE_A_PUBLIC_KEY, public[:]),
		nl.NewRtAttr(unix.WGDEVICE_A_LISTEN_PORT, binary.NativeEndian.AppendUint16(nil, 51820)),
		nl.NewRtAttr(unix.WGDEVICE_A_FWMARK, binary.NativeEndian.AppendUint32(nil, 0xca6c)),
		nl.NewRtAttr(9, binary.NativeEndian.AppendUint16(nil, 4)), nl.NewRtAttr(14, header), peers,
		nl.NewRtAttr(255, []byte("future attribute")),
	)
	continuation := nl.NewRtAttr(unix.WGDEVICE_A_PEERS|unix.NLA_F_NESTED, nil)
	peer = continuation.AddRtAttr(unix.NLA_F_NESTED, nil)
	peer.AddRtAttr(unix.WGPEER_A_PUBLIC_KEY, public[:])
	peer.AddChild(awgTestIPs("fd00::2", 128))
	return [][]byte{first, awgTestReply(version, nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, []byte("awg0\x00")), continuation)}
}

func TestAmneziaWGNativeDeviceVersionsAndMultipartDump(t *testing.T) {
	for version := uint8(1); version <= 3; version++ {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			client := &AmneziaWGClient{version: version, linkByName: awgKernelLink,
				exchange: func(command uint8, flags uint16, attrs []*nl.RtAttr) ([][]byte, error) {
					require.Equal(t, uint8(0), command)
					require.Equal(t, uint16(unix.NLM_F_DUMP), flags)
					require.Equal(t, []byte("awg0\x00"), awgTestAttrMap(t, awgTestSerialize(attrs))[unix.WGDEVICE_A_IFNAME])
					return awgTestDeviceReplies(t, version), nil
				},
			}
			device, err := client.Device("awg0")
			require.NoError(t, err)
			require.Equal(t, awgTestPrivateKey, device.PrivateKey.String())
			require.Equal(t, awgTestPublicKey, device.PublicKey.String())
			require.Equal(t, wgtypes.LinuxKernel, device.Type)
			require.Equal(t, 51820, device.ListenPort)
			require.Equal(t, 51820, device.FirewallMark)
			require.Len(t, device.Peers, 1)
			peer := device.Peers[0]
			require.Len(t, peer.AllowedIPs, 2)
			require.Equal(t, "10.0.0.2/32", peer.AllowedIPs[0].String())
			require.Equal(t, "fd00::2/128", peer.AllowedIPs[1].String())
			require.Equal(t, "[2001:db8::1]:1234", peer.Endpoint.String())
			require.Equal(t, time.Unix(1700000000, 42), peer.LastHandshakeTime)
			require.Equal(t, int64(123), peer.ReceiveBytes)
			require.Equal(t, int64(456), peer.TransmitBytes)
			require.Equal(t, 25*time.Second, peer.PersistentKeepaliveInterval)
			options, err := client.AmneziaWGOptions("awg0")
			require.NoError(t, err)
			header := "100"
			if version >= 2 {
				header = "100-200"
			}
			require.Equal(t, domain.AmneziaWGOptions{"H1": header, "Jc": "4"}, options)
		})
	}
}

func TestAmneziaWGKernelSupportWithoutTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	missing := errors.New("family not found")
	_, err := amneziaWGFamily(func(name string) (*netlink.GenlFamily, error) {
		require.Equal(t, "amneziawg", name)
		return nil, missing
	})
	require.ErrorIs(t, err, missing)
	require.ErrorContains(t, err, "modprobe amneziawg")
	for _, version := range []uint32{1, 2, 3} {
		family, err := amneziaWGFamily(func(string) (*netlink.GenlFamily, error) { return &netlink.GenlFamily{ID: 42, Version: version}, nil })
		require.NoError(t, err)
		require.Equal(t, version, family.Version)
	}
	_, err = amneziaWGFamily(func(string) (*netlink.GenlFamily, error) { return &netlink.GenlFamily{Version: 4}, nil })
	require.ErrorContains(t, err, "unsupported")
}

func TestAmneziaWGNativeConfigurationFlagsAndExplicitZeroes(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	key, err := wgtypes.ParseKey(awgTestPublicKey)
	require.NoError(t, err)
	private, err := wgtypes.ParseKey(awgTestPrivateKey)
	require.NoError(t, err)
	var captured []byte
	client := &AmneziaWGClient{version: 3, linkByName: awgKernelLink,
		exchange: func(command uint8, flags uint16, attrs []*nl.RtAttr) ([][]byte, error) {
			require.Equal(t, uint8(1), command, "configuration must not read and rewrite existing peers")
			require.Equal(t, uint16(unix.NLM_F_ACK), flags)
			captured = awgTestSerialize(attrs)
			return nil, nil
		},
	}
	zero, mark, keepalive := 0, -1, time.Duration(0)
	require.NoError(t, client.ConfigureAmneziaWGDevice("awg0", wgtypes.Config{
		PrivateKey: &private, ListenPort: &zero, FirewallMark: &mark,
		Peers: []wgtypes.PeerConfig{{PublicKey: key, UpdateOnly: true, ReplaceAllowedIPs: true, PresharedKey: &wgtypes.Key{}, PersistentKeepaliveInterval: &keepalive}},
	}, domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4"}))
	attrs := awgTestAttrMap(t, captured)
	require.Equal(t, private[:], attrs[unix.WGDEVICE_A_PRIVATE_KEY])
	require.Equal(t, []byte{0, 0}, attrs[unix.WGDEVICE_A_LISTEN_PORT])
	require.Equal(t, []byte{255, 255, 255, 255}, attrs[unix.WGDEVICE_A_FWMARK])
	require.NotContains(t, attrs, uint16(unix.WGDEVICE_A_FLAGS))
	peers, err := nl.ParseRouteAttr(attrs[unix.WGDEVICE_A_PEERS])
	require.NoError(t, err)
	require.Len(t, peers, 1)
	peer := awgTestAttrMap(t, peers[0].Value)
	require.Equal(t, uint32(unix.WGPEER_F_UPDATE_ONLY|unix.WGPEER_F_REPLACE_ALLOWEDIPS), binary.NativeEndian.Uint32(peer[unix.WGPEER_A_FLAGS]))
	require.Equal(t, make([]byte, 32), peer[unix.WGPEER_A_PRESHARED_KEY])
	require.Equal(t, make([]byte, 4), peer[unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL])
	require.NotContains(t, peer, uint16(unix.WGPEER_A_ALLOWEDIPS))
	// Removal is a native peer flag, and needs no extra process or state read.
	require.NoError(t, client.ConfigureDevice("awg0", wgtypes.Config{Peers: []wgtypes.PeerConfig{{PublicKey: key, Remove: true}}}))
	peers, err = nl.ParseRouteAttr(awgTestAttrMap(t, captured)[unix.WGDEVICE_A_PEERS])
	require.NoError(t, err)
	require.Equal(t, uint32(unix.WGPEER_F_REMOVE_ME), binary.NativeEndian.Uint32(awgTestAttrMap(t, peers[0].Value)[unix.WGPEER_A_FLAGS]))
}

func TestAmneziaWGNativeBatchingPreservesReplacementSemantics(t *testing.T) {
	key, err := wgtypes.ParseKey(awgTestPublicKey)
	require.NoError(t, err)
	ips := make([]net.IPNet, 4096)
	for i := range ips {
		ips[i] = net.IPNet{IP: net.ParseIP(fmt.Sprintf("fd00::%x", i)), Mask: net.CIDRMask(128, 128)}
	}
	interval := 25 * time.Second
	cfg := wgtypes.Config{ReplacePeers: true, Peers: []wgtypes.PeerConfig{{PublicKey: key, UpdateOnly: true, ReplaceAllowedIPs: true, PersistentKeepaliveInterval: &interval, AllowedIPs: ips}}}
	batches, err := amneziaWGConfigBatches("awg0", cfg, domain.AmneziaWGOptions{"Jc": "4"}, 3)
	require.NoError(t, err)
	require.Greater(t, len(batches), 1)
	firstPeer, count := true, 0
	for i, batch := range batches {
		data := awgTestSerialize(batch)
		require.LessOrEqual(t, len(data)+4, awgMaxPayload)
		attrs := awgTestAttrMap(t, data)
		if i == 0 {
			require.Equal(t, uint32(unix.WGDEVICE_F_REPLACE_PEERS), binary.NativeEndian.Uint32(attrs[unix.WGDEVICE_A_FLAGS]))
			require.Contains(t, attrs, uint16(9))
		} else {
			require.NotContains(t, attrs, uint16(unix.WGDEVICE_A_FLAGS))
			require.NotContains(t, attrs, uint16(9))
		}
		peers, err := nl.ParseRouteAttr(attrs[unix.WGDEVICE_A_PEERS])
		require.NoError(t, err)
		for _, raw := range peers {
			peer := awgTestAttrMap(t, raw.Value)
			flags := binary.NativeEndian.Uint32(peer[unix.WGPEER_A_FLAGS])
			require.NotZero(t, flags&unix.WGPEER_F_UPDATE_ONLY)
			require.Equal(t, firstPeer, flags&unix.WGPEER_F_REPLACE_ALLOWEDIPS != 0)
			if firstPeer {
				require.Equal(t, uint32(25<<16|25), binary.NativeEndian.Uint32(peer[unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL]))
			} else {
				require.NotContains(t, peer, uint16(unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL))
			}
			entries, err := nl.ParseRouteAttr(peer[unix.WGPEER_A_ALLOWEDIPS])
			require.NoError(t, err)
			count += len(entries)
			firstPeer = false
		}
	}
	require.Equal(t, len(ips), count)
	require.True(t, cfg.ReplacePeers)
	require.True(t, cfg.Peers[0].ReplaceAllowedIPs)
	require.Len(t, cfg.Peers[0].AllowedIPs, 4096)
}

func TestAmneziaWGNativeLinkIsolationAndErrors(t *testing.T) {
	for _, kind := range []string{"wireguard", "tun"} {
		client := &AmneziaWGClient{version: 3,
			linkByName: func(name string) (netlink.Link, error) {
				return &netlink.GenericLink{Name: name, LinkType: kind}, nil
			},
			exchange: func(uint8, uint16, []*nl.RtAttr) ([][]byte, error) {
				t.Fatal("must not touch other link types")
				return nil, nil
			},
		}
		_, err := client.Device("wg0")
		require.ErrorContains(t, err, "not an AmneziaWG kernel device")
		require.Error(t, client.ConfigureDevice("wg0", wgtypes.Config{}))
	}
	client := &AmneziaWGClient{version: 3, linkByName: awgKernelLink,
		exchange: func(uint8, uint16, []*nl.RtAttr) ([][]byte, error) { return nil, unix.ENODEV },
	}
	_, err := client.Device("missing")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorIs(t, client.ConfigureDevice("missing", wgtypes.Config{}), os.ErrNotExist)
	client.links = func() ([]netlink.Link, error) {
		wg, _ := awgKernelLink("gone")
		return []netlink.Link{&netlink.Dummy{Name: "ordinary"}, wg}, nil
	}
	devices, err := client.Devices()
	require.NoError(t, err)
	require.Empty(t, devices)
}

func TestAmneziaWGRejectsInvalidConfigurationBeforeSending(t *testing.T) {
	client := &AmneziaWGClient{version: 1, linkByName: awgKernelLink,
		exchange: func(uint8, uint16, []*nl.RtAttr) ([][]byte, error) {
			t.Fatal("invalid input sent to kernel")
			return nil, nil
		},
	}
	for _, options := range []domain.AmneziaWGOptions{{"H1": "100-200"}, {"I1": "<b 0x12>"}, {"RekeyTimeout": "5"}, {"Jc": "65536"}, {"I1": "bad\n[Peer]"}} {
		require.Error(t, client.ConfigureAmneziaWGDevice("awg0", wgtypes.Config{}, options))
	}
	badPort := 65536
	require.Error(t, client.ConfigureDevice("awg0", wgtypes.Config{ListenPort: &badPort}))
	require.Error(t, client.ConfigureDevice("awg0", wgtypes.Config{Peers: []wgtypes.PeerConfig{{Endpoint: &net.UDPAddr{IP: nil, Port: 1}}}}))
}

func TestAmneziaWGNativeOptionEncodings(t *testing.T) {
	for _, test := range []struct {
		version uint8
		value   string
		wire    []byte
	}{
		{1, "305419896", binary.NativeEndian.AppendUint32(nil, 0x12345678)},
		{2, "100-200", []byte("100-200\x00")},
		{3, "100-200", binary.NativeEndian.AppendUint64(nil, 200<<32|100)},
	} {
		attrs, err := encodeAmneziaWGOptions(domain.AmneziaWGOptions{"H1": test.value}, test.version)
		require.NoError(t, err)
		require.Equal(t, test.wire, awgTestAttrMap(t, awgTestSerialize(attrs))[14])
	}
	options := domain.AmneziaWGOptions{"Jc": "4", "S3": "16", "I1": "<b 0x1234>", "ContentPaddingAddition": "10-20", "RekeyTimeout": "5", "RandomTrailers": "on", "DisableCookies": "off", "HeaderProtectionKey": awgTestPrivateKey}
	attrs, err := encodeAmneziaWGOptions(options, 3)
	require.NoError(t, err)
	_, parsed, err := parseAmneziaWGDevice([][]byte{awgTestReply(3, attrs...)}, 3)
	require.NoError(t, err)
	require.Equal(t, options, parsed)
	encoded := awgTestAttrMap(t, awgTestSerialize(attrs))
	require.Equal(t, uint32(20<<16|10), binary.NativeEndian.Uint32(encoded[27]))
	require.Equal(t, uint32(5<<16|5), binary.NativeEndian.Uint32(encoded[29]))
}

func TestAmneziaWGSockaddrsAndTimes(t *testing.T) {
	for _, endpoint := range []*net.UDPAddr{
		{IP: net.ParseIP("192.0.2.1"), Port: 51820},
		{IP: net.ParseIP("2001:db8::1"), Port: 1234},
		{IP: net.ParseIP("fe80::1"), Port: 1234, Zone: "3"},
	} {
		data, err := encodeAmneziaWGSockaddr(endpoint)
		require.NoError(t, err)
		parsed, err := parseAmneziaWGSockaddr(data)
		require.NoError(t, err)
		require.Equal(t, endpoint.String(), parsed.String())
	}
	zero, err := parseAmneziaWGTime(make([]byte, 16))
	require.NoError(t, err)
	require.True(t, zero.IsZero())
	parsed, err := parseAmneziaWGTime(append(binary.NativeEndian.AppendUint32(nil, 1700000000), binary.NativeEndian.AppendUint32(nil, 42)...))
	require.NoError(t, err)
	require.Equal(t, time.Unix(1700000000, 42), parsed)
}

func TestAmneziaWGMalformedMessages(t *testing.T) {
	for _, data := range [][]byte{
		{}, {0, 1, 0}, {0, 1, 0, 0, 1}, {0, 1, 0, 0, 3, 0, 0, 0},
		awgTestReply(1, nl.NewRtAttr(unix.WGDEVICE_A_PRIVATE_KEY, []byte{1})),
		awgTestReply(3, nl.NewRtAttr(14, []byte{1})),
	} {
		_, _, err := parseAmneziaWGDevice([][]byte{data}, 3)
		require.Error(t, err)
		require.NotContains(t, err.Error(), awgTestPrivateKey)
	}
	_, err := parseAmneziaWGPeer(nil, 3)
	require.ErrorContains(t, err, "public key")
	_, err = parseAmneziaWGAllowedIPs(nl.NewRtAttr(unix.NLA_F_NESTED, nil).Serialize())
	require.Error(t, err)
	_, err = parseAmneziaWGTime([]byte{1})
	require.Error(t, err)
	_, err = parseAmneziaWGSockaddr([]byte{1})
	require.Error(t, err)
}

func FuzzAmneziaWGDeviceDecoder(f *testing.F) {
	f.Add([]byte{0, 3, 0, 0})
	f.Add(awgTestReply(3, nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, []byte("awg0\x00"))))
	f.Fuzz(func(t *testing.T, data []byte) { _, _, _ = parseAmneziaWGDevice([][]byte{data}, 3) })
}
