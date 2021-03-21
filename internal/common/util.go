package common

import (
	"fmt"
	"net"
	"strings"
)

// BroadcastAddr returns the last address in the given network, or the broadcast address.
func BroadcastAddr(n *net.IPNet) net.IP {
	// The golang net package doesn't make it easy to calculate the broadcast address. :(
	var broadcast net.IP
	if len(n.IP) == 4 {
		broadcast = net.ParseIP("0.0.0.0").To4()
	} else {
		broadcast = net.ParseIP("::")
	}
	for i := 0; i < len(n.IP); i++ {
		broadcast[i] = n.IP[i] | ^n.Mask[i]
	}
	return broadcast
}

//  http://play.golang.org/p/m8TNTtygK0
func IncreaseIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// IsIPv6 check if given ip is IPv6
func IsIPv6(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

// ParseStringList converts a comma separated string into a list of strings.
// It also trims spaces from each element of the list.
func ParseStringList(lst string) []string {
	tokens := strings.Split(lst, ",")
	validatedTokens := make([]string, 0, len(tokens))
	for i := range tokens {
		tokens[i] = strings.TrimSpace(tokens[i])
		if tokens[i] != "" {
			validatedTokens = append(validatedTokens, tokens[i])
		}
	}

	return validatedTokens
}

// ListToString converts a list of strings into a comma separated string.
func ListToString(lst []string) string {
	return strings.Join(lst, ", ")
}

// ListContains checks if a needle exists in the given list.
func ListContains(lst []string, needle string) bool {
	for _, entry := range lst {
		if entry == needle {
			return true
		}
	}
	return false
}

// https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
