package lowlevel

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/domain"
)

const (
	awgGetDevice  uint8 = 0
	awgSetDevice  uint8 = 1
	awgMaxPayload       = 60000
	awgIPChunk          = 256
)

func amneziaWGConfigBatches(name string, cfg wgtypes.Config, options domain.AmneziaWGOptions, version uint8) ([][]*nl.RtAttr, error) {
	if len(name) == 0 || len(name) >= unix.IFNAMSIZ || strings.ContainsRune(name, 0) {
		return nil, fmt.Errorf("invalid AmneziaWG interface name")
	}
	attrs := []*nl.RtAttr{nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, nl.ZeroTerminated(name))}
	if cfg.PrivateKey != nil {
		attrs = append(attrs, nl.NewRtAttr(unix.WGDEVICE_A_PRIVATE_KEY, cfg.PrivateKey[:]))
	}
	if cfg.ListenPort != nil {
		if *cfg.ListenPort < 0 || *cfg.ListenPort > 65535 {
			return nil, fmt.Errorf("invalid AmneziaWG listen port")
		}
		attrs = append(attrs, nl.NewRtAttr(unix.WGDEVICE_A_LISTEN_PORT, awgNumber(uint64(*cfg.ListenPort), 2)))
	}
	if cfg.FirewallMark != nil {
		attrs = append(attrs, nl.NewRtAttr(unix.WGDEVICE_A_FWMARK, awgNumber(uint64(uint32(*cfg.FirewallMark)), 4)))
	}
	if cfg.ReplacePeers {
		attrs = append(attrs, nl.NewRtAttr(unix.WGDEVICE_A_FLAGS, awgNumber(unix.WGDEVICE_F_REPLACE_PEERS, 4)))
	}
	custom, err := encodeAmneziaWGOptions(options, version)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, custom...)
	var batches [][]*nl.RtAttr
	var peers *nl.RtAttr
	size := nl.SizeofGenlmsg
	for _, attr := range attrs {
		size += len(attr.Serialize())
	}
	flush := func() {
		if peers != nil {
			attrs = append(attrs, peers)
		}
		batches = append(batches, attrs)
		// Destructive replacement flags and interface options belong only in
		// the first message. Later messages append peer/IP continuations.
		attrs = []*nl.RtAttr{nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, nl.ZeroTerminated(name))}
		size = nl.SizeofGenlmsg + len(attrs[0].Serialize())
		peers = nil
	}
	for _, peer := range cfg.Peers {
		for start, first := 0, true; first || start < len(peer.AllowedIPs); first = false {
			end := min(start+awgIPChunk, len(peer.AllowedIPs))
			chunk, err := encodeAmneziaWGPeer(peer, peer.AllowedIPs[start:end], first, version)
			if err != nil {
				return nil, err
			}
			length := len(chunk.Serialize())
			if size+length+unix.SizeofRtAttr > awgMaxPayload {
				flush()
			}
			if peers == nil {
				peers = nl.NewRtAttr(unix.WGDEVICE_A_PEERS|unix.NLA_F_NESTED, nil)
				size += unix.SizeofRtAttr
			}
			peers.AddChild(chunk)
			size += length
			start = end
		}
	}
	flush()
	return batches, nil
}

func encodeAmneziaWGPeer(peer wgtypes.PeerConfig, ips []net.IPNet, first bool, version uint8) (*nl.RtAttr, error) {
	attr := nl.NewRtAttr(unix.NLA_F_NESTED, nil)
	attr.AddRtAttr(unix.WGPEER_A_PUBLIC_KEY, peer.PublicKey[:])
	var flags uint32
	if peer.Remove {
		flags |= unix.WGPEER_F_REMOVE_ME
	}
	if peer.UpdateOnly {
		flags |= unix.WGPEER_F_UPDATE_ONLY
	}
	if first && peer.ReplaceAllowedIPs {
		flags |= unix.WGPEER_F_REPLACE_ALLOWEDIPS
	}
	if flags != 0 {
		attr.AddRtAttr(unix.WGPEER_A_FLAGS, awgNumber(uint64(flags), 4))
	}
	if first {
		if peer.PresharedKey != nil {
			attr.AddRtAttr(unix.WGPEER_A_PRESHARED_KEY, peer.PresharedKey[:])
		}
		if peer.Endpoint != nil {
			data, err := encodeAmneziaWGSockaddr(peer.Endpoint)
			if err != nil {
				return nil, err
			}
			attr.AddRtAttr(unix.WGPEER_A_ENDPOINT, data)
		}
		if peer.PersistentKeepaliveInterval != nil {
			interval := *peer.PersistentKeepaliveInterval
			if interval < 0 || interval > 65535*time.Second || interval%time.Second != 0 {
				return nil, fmt.Errorf("invalid AmneziaWG keepalive interval")
			}
			value, size := uint64(interval/time.Second), 2
			if version >= 3 {
				value |= value << 16
				size = 4
			}
			attr.AddRtAttr(unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL, awgNumber(value, size))
		}
	}
	if len(ips) != 0 {
		allowed := attr.AddRtAttr(unix.WGPEER_A_ALLOWEDIPS|unix.NLA_F_NESTED, nil)
		for _, ip := range ips {
			address, family, bits := ip.IP.To4(), uint64(unix.AF_INET), 32
			if address == nil {
				address, family, bits = ip.IP.To16(), unix.AF_INET6, 128
			}
			ones, maskBits := ip.Mask.Size()
			if address == nil || maskBits != bits {
				return nil, fmt.Errorf("invalid AmneziaWG allowed IP")
			}
			entry := allowed.AddRtAttr(unix.NLA_F_NESTED, nil)
			entry.AddRtAttr(unix.WGALLOWEDIP_A_FAMILY, awgNumber(family, 2))
			entry.AddRtAttr(unix.WGALLOWEDIP_A_IPADDR, address)
			entry.AddRtAttr(unix.WGALLOWEDIP_A_CIDR_MASK, []byte{byte(ones)})
		}
	}
	return attr, nil
}

func encodeAmneziaWGSockaddr(endpoint *net.UDPAddr) ([]byte, error) {
	if endpoint.Port < 0 || endpoint.Port > 65535 {
		return nil, fmt.Errorf("invalid AmneziaWG endpoint port")
	}
	if ip := endpoint.IP.To4(); ip != nil {
		data := make([]byte, unix.SizeofSockaddrInet4)
		nl.NativeEndian().PutUint16(data, unix.AF_INET)
		binary.BigEndian.PutUint16(data[2:], uint16(endpoint.Port))
		copy(data[4:], ip)
		return data, nil
	}
	ip := endpoint.IP.To16()
	if ip == nil {
		return nil, fmt.Errorf("invalid AmneziaWG endpoint IP")
	}
	data := make([]byte, unix.SizeofSockaddrInet6)
	nl.NativeEndian().PutUint16(data, unix.AF_INET6)
	binary.BigEndian.PutUint16(data[2:], uint16(endpoint.Port))
	copy(data[8:], ip)
	if endpoint.Zone != "" {
		index, err := strconv.ParseUint(endpoint.Zone, 10, 32)
		if err != nil {
			iface, err := net.InterfaceByName(endpoint.Zone)
			if err != nil {
				return nil, fmt.Errorf("invalid AmneziaWG endpoint zone")
			}
			index = uint64(iface.Index)
		}
		nl.NativeEndian().PutUint32(data[24:], uint32(index))
	}
	return data, nil
}

// Validate lengths before using netlink's attribute decoder, including padding
// that would otherwise let a truncated attribute panic during decoding.
func amneziaWGAttrs(data []byte) ([]syscall.NetlinkRouteAttr, error) {
	for rest := data; len(rest) != 0; {
		if len(rest) < unix.SizeofRtAttr {
			return nil, fmt.Errorf("truncated AmneziaWG attribute")
		}
		length := int(nl.NativeEndian().Uint16(rest))
		padded := (length + 3) &^ 3
		if length < unix.SizeofRtAttr || padded > len(rest) {
			return nil, fmt.Errorf("invalid AmneziaWG attribute length")
		}
		rest = rest[padded:]
	}
	return nl.ParseRouteAttr(data)
}

func parseAmneziaWGDevice(messages [][]byte, version uint8) (*wgtypes.Device, domain.AmneziaWGOptions, error) {
	device := &wgtypes.Device{Type: wgtypes.LinuxKernel}
	options := domain.AmneziaWGOptions{}
	known := make(map[wgtypes.Key]int)
	for _, message := range messages {
		if len(message) < nl.SizeofGenlmsg || message[0] != awgGetDevice {
			return nil, nil, fmt.Errorf("invalid AmneziaWG message header")
		}
		attrs, err := amneziaWGAttrs(message[nl.SizeofGenlmsg:])
		if err != nil {
			return nil, nil, err
		}
		for _, attr := range attrs {
			kind, data := attr.Attr.Type&0x3fff, attr.Value
			switch kind {
			case unix.WGDEVICE_A_IFNAME:
				var name string
				name, err = awgString(data)
				if device.Name != "" && name != device.Name {
					return nil, nil, fmt.Errorf("mixed AmneziaWG interfaces in dump")
				}
				device.Name = name
			case unix.WGDEVICE_A_PRIVATE_KEY:
				device.PrivateKey, err = wgtypes.NewKey(data)
			case unix.WGDEVICE_A_PUBLIC_KEY:
				device.PublicKey, err = wgtypes.NewKey(data)
			case unix.WGDEVICE_A_LISTEN_PORT:
				var v uint64
				v, err = awgReadNumber(data, 2)
				device.ListenPort = int(v)
			case unix.WGDEVICE_A_FWMARK:
				var v uint64
				v, err = awgReadNumber(data, 4)
				device.FirewallMark = int(v)
			case unix.WGDEVICE_A_PEERS:
				var peers []syscall.NetlinkRouteAttr
				peers, err = amneziaWGAttrs(data)
				for _, raw := range peers {
					if err != nil {
						break
					}
					var peer wgtypes.Peer
					peer, err = parseAmneziaWGPeer(raw.Value, version)
					if err != nil {
						break
					}
					if index, ok := known[peer.PublicKey]; ok {
						device.Peers[index].AllowedIPs = append(device.Peers[index].AllowedIPs, peer.AllowedIPs...)
					} else {
						known[peer.PublicKey] = len(device.Peers)
						device.Peers = append(device.Peers, peer)
					}
				}
			default:
				for _, spec := range awgOptions {
					if int(kind) == spec.attr {
						options[spec.name], err = spec.decode(data, version)
						break
					}
				}
			}
			if err != nil {
				return nil, nil, fmt.Errorf("invalid AmneziaWG attribute %d", kind)
			}
		}
	}
	if err := options.Validate(); err != nil {
		return nil, nil, err
	}
	return device, options, nil
}

func parseAmneziaWGPeer(data []byte, version uint8) (wgtypes.Peer, error) {
	peer := wgtypes.Peer{ProtocolVersion: 1}
	attrs, err := amneziaWGAttrs(data)
	if err != nil {
		return peer, err
	}
	keyFound := false
	for _, attr := range attrs {
		kind, value := attr.Attr.Type&0x3fff, attr.Value
		switch kind {
		case unix.WGPEER_A_PUBLIC_KEY:
			peer.PublicKey, err = wgtypes.NewKey(value)
			keyFound = true
		case unix.WGPEER_A_PRESHARED_KEY:
			peer.PresharedKey, err = wgtypes.NewKey(value)
		case unix.WGPEER_A_ENDPOINT:
			peer.Endpoint, err = parseAmneziaWGSockaddr(value)
		case unix.WGPEER_A_PERSISTENT_KEEPALIVE_INTERVAL:
			size := 2
			if version >= 3 {
				size = 4
			}
			var v uint64
			v, err = awgReadNumber(value, size)
			// The portal exposes a single interval; report a range's lower bound.
			peer.PersistentKeepaliveInterval = time.Duration(v&0xffff) * time.Second
		case unix.WGPEER_A_LAST_HANDSHAKE_TIME:
			peer.LastHandshakeTime, err = parseAmneziaWGTime(value)
		case unix.WGPEER_A_RX_BYTES:
			var v uint64
			v, err = awgReadNumber(value, 8)
			peer.ReceiveBytes = int64(v)
		case unix.WGPEER_A_TX_BYTES:
			var v uint64
			v, err = awgReadNumber(value, 8)
			peer.TransmitBytes = int64(v)
		case unix.WGPEER_A_PROTOCOL_VERSION:
			var v uint64
			v, err = awgReadNumber(value, 4)
			peer.ProtocolVersion = int(v)
		case unix.WGPEER_A_ALLOWEDIPS:
			peer.AllowedIPs, err = parseAmneziaWGAllowedIPs(value)
		}
		if err != nil {
			return peer, fmt.Errorf("invalid AmneziaWG peer attribute %d", kind)
		}
	}
	if !keyFound {
		return peer, fmt.Errorf("missing AmneziaWG peer public key")
	}
	return peer, nil
}

func parseAmneziaWGAllowedIPs(data []byte) ([]net.IPNet, error) {
	entries, err := amneziaWGAttrs(data)
	if err != nil {
		return nil, err
	}
	var ips []net.IPNet
	for _, entry := range entries {
		attrs, err := amneziaWGAttrs(entry.Value)
		if err != nil {
			return nil, err
		}
		var family, prefix uint64
		var address net.IP
		familyFound, prefixFound := false, false
		for _, attr := range attrs {
			switch attr.Attr.Type & 0x3fff {
			case unix.WGALLOWEDIP_A_FAMILY:
				family, err = awgReadNumber(attr.Value, 2)
				familyFound = true
			case unix.WGALLOWEDIP_A_CIDR_MASK:
				prefix, err = awgReadNumber(attr.Value, 1)
				prefixFound = true
			case unix.WGALLOWEDIP_A_IPADDR:
				address = append(net.IP(nil), attr.Value...)
			}
			if err != nil {
				return nil, err
			}
		}
		bits := 0
		if family == unix.AF_INET && len(address) == net.IPv4len {
			bits = 32
		}
		if family == unix.AF_INET6 && len(address) == net.IPv6len {
			bits = 128
		}
		if !familyFound || !prefixFound || bits == 0 || prefix > uint64(bits) {
			return nil, fmt.Errorf("invalid AmneziaWG allowed IP")
		}
		ips = append(ips, net.IPNet{IP: address, Mask: net.CIDRMask(int(prefix), bits)})
	}
	return ips, nil
}

func parseAmneziaWGSockaddr(data []byte) (*net.UDPAddr, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("truncated AmneziaWG endpoint")
	}
	family := nl.NativeEndian().Uint16(data)
	endpoint := &net.UDPAddr{Port: int(binary.BigEndian.Uint16(data[2:]))}
	switch {
	case family == unix.AF_INET && len(data) == unix.SizeofSockaddrInet4:
		endpoint.IP = append(net.IP(nil), data[4:8]...)
	case family == unix.AF_INET6 && len(data) == unix.SizeofSockaddrInet6:
		endpoint.IP = append(net.IP(nil), data[8:24]...)
		if zone := nl.NativeEndian().Uint32(data[24:]); zone != 0 {
			endpoint.Zone = strconv.FormatUint(uint64(zone), 10)
		}
	default:
		return nil, fmt.Errorf("invalid AmneziaWG endpoint")
	}
	return endpoint, nil
}

func parseAmneziaWGTime(data []byte) (time.Time, error) {
	var seconds, nanos int64
	switch len(data) {
	case 8:
		seconds = int64(int32(nl.NativeEndian().Uint32(data)))
		nanos = int64(int32(nl.NativeEndian().Uint32(data[4:])))
	case 16:
		seconds = int64(nl.NativeEndian().Uint64(data))
		nanos = int64(nl.NativeEndian().Uint64(data[8:]))
	default:
		return time.Time{}, fmt.Errorf("invalid AmneziaWG handshake timestamp")
	}
	if seconds < 0 || nanos < 0 || nanos >= int64(time.Second) {
		return time.Time{}, fmt.Errorf("invalid AmneziaWG handshake timestamp")
	}
	if seconds == 0 && nanos == 0 {
		return time.Time{}, nil
	}
	return time.Unix(seconds, nanos), nil
}
