package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// SignalAwareContext returns a context that gets closed once a given signal is retrieved.
// By default, the following signals are handled: syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP
func SignalAwareContext(ctx context.Context, sig ...os.Signal) context.Context {
	c := make(chan os.Signal, 1)
	if len(sig) == 0 {
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	} else {

		signal.Notify(c, sig...)
	}
	signalCtx, cancel := context.WithCancel(ctx)

	// Attach signal handlers to context
	go func() {
		select {
		case <-ctx.Done():
			// normal shutdown, quit go routine
		case <-c:
			cancel() // cancel the context
		}

		// cleanup
		signal.Stop(c)
		close(c)
	}()

	return signalCtx
}

// AssertNoError panics if the given error is not nil.
func AssertNoError(err error) {
	if err != nil {
		panic(err)
	}
}

// ByteCountSI returns the byte count as string, see: https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
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

// MapDefaultString returns the string value for the given key or a default value
func MapDefaultString(m map[string]interface{}, key string, dflt string) string {
	if m == nil {
		return dflt
	}
	if tmp, ok := m[key]; !ok {
		return dflt
	} else {
		switch v := tmp.(type) {
		case string:
			return v
		case nil:
			return dflt
		default:
			return fmt.Sprintf("%v", v)
		}
	}
}

// UniqueStringSlice removes duplicates in the given string slice
func UniqueStringSlice(slice []string) []string {
	keys := make(map[string]struct{})
	uniqueSlice := make([]string, 0, len(slice))
	for _, entry := range slice {
		if _, exists := keys[entry]; !exists {
			keys[entry] = struct{}{}
			uniqueSlice = append(uniqueSlice, entry)
		}
	}
	return uniqueSlice
}

func SliceContains[T comparable](slice []T, needle T) bool {
	for _, elem := range slice {
		if elem == needle {
			return true
		}
	}

	return false
}

func SliceString(str string) []string {
	strParts := strings.Split(str, ",")
	stringSlice := make([]string, 0, len(strParts))

	for _, s := range strParts {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			stringSlice = append(stringSlice, trimmed)
		}
	}

	return stringSlice
}

func SliceToString(slice []string) string {
	return strings.Join(slice, ",")
}

func TruncateString(s string, max int) string {
	if max > len(s) {
		return s
	}
	return s[:max]
}

func BoolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
