package lowlevel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink/nl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/domain"
)

// Attribute numbers follow the official AmneziaWG UAPI, which extends the
// WireGuard attributes without changing their IDs.
// https://github.com/amnezia-vpn/amneziawg-tools/blob/ee0f0a9aa34ff0a0da4b3433b9512781cfe02843/src/uapi/linux/linux/wireguard.h
const (
	awgOptionU16 = iota
	awgOptionHeader
	awgOptionString
	awgOptionKey
	awgOptionRange16
	awgOptionBool
)

type awgOption struct {
	name       string
	attr       int
	kind       int
	minVersion uint8
}

var awgOptions = []awgOption{
	{"Jc", 9, awgOptionU16, 1}, {"Jmin", 10, awgOptionU16, 1}, {"Jmax", 11, awgOptionU16, 1},
	{"S1", 12, awgOptionU16, 1}, {"S2", 13, awgOptionU16, 1},
	{"H1", 14, awgOptionHeader, 1}, {"H2", 15, awgOptionHeader, 1},
	{"H3", 16, awgOptionHeader, 1}, {"H4", 17, awgOptionHeader, 1},
	// Attribute 18 describes an unknown peer notification, not an option.
	{"S3", 19, awgOptionU16, 2}, {"S4", 20, awgOptionU16, 2},
	{"I1", 21, awgOptionString, 2}, {"I2", 22, awgOptionString, 2},
	{"I3", 23, awgOptionString, 2}, {"I4", 24, awgOptionString, 2}, {"I5", 25, awgOptionString, 2},
	{"HeaderProtectionKey", 26, awgOptionKey, 3},
	{"ContentPaddingAddition", 27, awgOptionRange16, 3}, {"RekeyAfterTime", 28, awgOptionRange16, 3},
	{"RekeyTimeout", 29, awgOptionRange16, 3}, {"RejectAfterTime", 30, awgOptionRange16, 3},
	{"KeepaliveTimeout", 31, awgOptionRange16, 3}, {"MaxHandshakeAttempts", 32, awgOptionRange16, 3},
	{"RandomTrailers", 33, awgOptionBool, 3}, {"DisableCookies", 34, awgOptionBool, 3},
}

func encodeAmneziaWGOptions(options domain.AmneziaWGOptions, version uint8) ([]*nl.RtAttr, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	var attrs []*nl.RtAttr
	for _, spec := range awgOptions {
		value, ok := options[spec.name]
		if !ok {
			continue
		}
		if version < spec.minVersion {
			return nil, fmt.Errorf("AmneziaWG option %s requires kernel netlink version %d: %w", spec.name, spec.minVersion, domain.ErrInvalidData)
		}
		data, err := spec.encode(value, version)
		if err != nil {
			return nil, fmt.Errorf("invalid AmneziaWG option %s: %v: %w", spec.name, err, domain.ErrInvalidData)
		}
		attrs = append(attrs, nl.NewRtAttr(spec.attr, data))
	}
	return attrs, nil
}

func (o awgOption) encode(value string, version uint8) ([]byte, error) {
	switch o.kind {
	case awgOptionU16:
		v, err := strconv.ParseUint(value, 10, 16)
		return awgNumber(v, 2), err
	case awgOptionHeader:
		lo, hi, err := awgRange(value, 32)
		if err != nil {
			return nil, err
		}
		switch version {
		case 1:
			if lo != hi {
				return nil, fmt.Errorf("header ranges require AmneziaWG v2 or newer")
			}
			return awgNumber(lo, 4), nil
		case 2:
			return nl.ZeroTerminated(awgRangeString(lo, hi)), nil
		default:
			return awgNumber(lo|hi<<32, 8), nil
		}
	case awgOptionString:
		return nl.ZeroTerminated(value), nil
	case awgOptionKey:
		key, err := wgtypes.ParseKey(value)
		return key[:], err
	case awgOptionRange16:
		lo, hi, err := awgRange(value, 16)
		return awgNumber(lo|hi<<16, 4), err
	case awgOptionBool:
		if value == "1" || value == "on" {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	}
	return nil, fmt.Errorf("unknown AmneziaWG option kind")
}

func (o awgOption) decode(data []byte, version uint8) (string, error) {
	switch o.kind {
	case awgOptionU16:
		v, err := awgReadNumber(data, 2)
		return strconv.FormatUint(v, 10), err
	case awgOptionHeader:
		switch version {
		case 1:
			v, err := awgReadNumber(data, 4)
			return strconv.FormatUint(v, 10), err
		case 2:
			value, err := awgString(data)
			if err != nil {
				return "", err
			}
			lo, hi, err := awgRange(value, 32)
			return awgRangeString(lo, hi), err
		default:
			v, err := awgReadNumber(data, 8)
			return awgRangeString(v&0xffffffff, v>>32), err
		}
	case awgOptionString:
		return awgString(data)
	case awgOptionKey:
		key, err := wgtypes.NewKey(data)
		return key.String(), err
	case awgOptionRange16:
		v, err := awgReadNumber(data, 4)
		return awgRangeString(v&0xffff, v>>16), err
	case awgOptionBool:
		v, err := awgReadNumber(data, 1)
		if v != 0 {
			return "on", err
		}
		return "off", err
	}
	return "", fmt.Errorf("unknown AmneziaWG option kind")
}

func awgNumber(v uint64, size int) []byte {
	b := make([]byte, size)
	switch size {
	case 1:
		b[0] = byte(v)
	case 2:
		nl.NativeEndian().PutUint16(b, uint16(v))
	case 4:
		nl.NativeEndian().PutUint32(b, uint32(v))
	case 8:
		nl.NativeEndian().PutUint64(b, v)
	}
	return b
}

func awgReadNumber(b []byte, size int) (uint64, error) {
	if len(b) != size {
		return 0, fmt.Errorf("invalid AmneziaWG numeric attribute size")
	}
	switch size {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(nl.NativeEndian().Uint16(b)), nil
	case 4:
		return uint64(nl.NativeEndian().Uint32(b)), nil
	case 8:
		return nl.NativeEndian().Uint64(b), nil
	}
	return 0, fmt.Errorf("invalid AmneziaWG numeric size")
}

func awgString(data []byte) (string, error) {
	if len(data) == 0 || data[len(data)-1] != 0 || strings.IndexByte(string(data[:len(data)-1]), 0) >= 0 {
		return "", fmt.Errorf("invalid AmneziaWG string attribute")
	}
	return string(data[:len(data)-1]), nil
}

func awgRange(value string, bits int) (uint64, uint64, error) {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid range")
	}
	lo, err := strconv.ParseUint(parts[0], 10, bits)
	if err != nil {
		return 0, 0, err
	}
	hi := lo
	if len(parts) == 2 {
		hi, err = strconv.ParseUint(parts[1], 10, bits)
	}
	if err != nil || lo > hi {
		return 0, 0, fmt.Errorf("invalid range")
	}
	return lo, hi, nil
}

func awgRangeString(lo, hi uint64) string {
	if lo == hi {
		return strconv.FormatUint(lo, 10)
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}
