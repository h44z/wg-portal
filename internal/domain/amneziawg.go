package domain

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// AmneziaWGOptions contains [Interface] options understood by the official awg tool.
// See: https://docs.amnezia.org/documentation/amnezia-wg#configuration-parameters
type AmneziaWGOptions map[string]string

// Validate checks the custom parameters against the official limits.
func (o AmneziaWGOptions) Validate() error {
	if len(o) == 0 {
		return nil
	}
	for key, value := range o {
		if strings.ContainsAny(value, "\r\n\x00#") || len(value) > 4096 {
			return fmt.Errorf("invalid AmneziaWG option %s: %w", key, ErrInvalidData)
		}
		var err error
		switch key {
		case "Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4":
			_, err = strconv.ParseUint(value, 10, 16)
		case "H1", "H2", "H3", "H4":
			err = validateAmneziaWGRange(value, 32)
		case "ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout", "MaxHandshakeAttempts":
			err = validateAmneziaWGRange(value, 16)
		case "I1", "I2", "I3", "I4", "I5":
			// The kernel validates the CPS description; it must stay on one line.
			if value == "" {
				err = fmt.Errorf("empty CPS description")
			}
		case "HeaderProtectionKey":
			_, err = wgtypes.ParseKey(value)
		case "RandomTrailers", "DisableCookies":
			if value != "on" && value != "off" && value != "0" && value != "1" {
				err = fmt.Errorf("expected on/off or 0/1")
			}
		default:
			return fmt.Errorf("unknown AmneziaWG option %q: %w", key, ErrInvalidData)
		}
		if err != nil {
			return fmt.Errorf("invalid AmneziaWG option %s: %w", key, ErrInvalidData)
		}
	}
	if min, ok := o["Jmin"]; ok {
		if max, ok := o["Jmax"]; ok {
			lo, _ := strconv.ParseUint(min, 10, 16)
			hi, _ := strconv.ParseUint(max, 10, 16)
			if lo > hi {
				return fmt.Errorf("AmneziaWG Jmin exceeds Jmax: %w", ErrInvalidData)
			}
		}
	}
	return nil
}

func validateAmneziaWGRange(value string, bits int) error {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return fmt.Errorf("invalid range")
	}
	lo, err := strconv.ParseUint(parts[0], 10, bits)
	if err != nil {
		return err
	}
	if len(parts) == 2 {
		hi, err := strconv.ParseUint(parts[1], 10, bits)
		if err != nil || lo > hi {
			return fmt.Errorf("invalid range")
		}
	}
	return nil
}

// ConfigLines is deterministic so saved files and QR codes use the same options.
func (o AmneziaWGOptions) ConfigLines() string {
	var result strings.Builder
	for _, key := range slices.Sorted(maps.Keys(o)) {
		_, _ = fmt.Fprintf(&result, "%s = %s\n", key, o[key])
	}
	return result.String()
}
