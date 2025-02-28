package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
)

// LogClose closes the given Closer and logs any error that occurs
func LogClose(c io.Closer) {
	if err := c.Close(); err != nil {
		logrus.Errorf("error during Close(): %v", err)
	}
}

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

// MapDefaultString returns the string value for the given key or a default value
func MapDefaultString(m map[string]any, key string, dflt string) string {
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

// MapDefaultStringSlice returns the string slice value for the given key or a default value
func MapDefaultStringSlice(m map[string]any, key string, dflt []string) []string {
	if m == nil {
		return dflt
	}
	if tmp, ok := m[key]; !ok {
		return dflt
	} else {
		switch v := tmp.(type) {
		case []any:
			result := make([]string, 0, len(v))
			for _, elem := range v {
				switch vElem := elem.(type) {
				case string:
					result = append(result, vElem)
				default:
					result = append(result, fmt.Sprintf("%v", vElem))
				}
			}
			return result
		case []string:
			return v
		case string:
			return []string{v}
		case nil:
			return dflt
		default:
			return []string{fmt.Sprintf("%v", v)}
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

// SliceString returns a string slice from a comma-separated string
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

// SliceToString returns a comma-separated string from a string slice
func SliceToString(slice []string) string {
	return strings.Join(slice, ",")
}

// TruncateString returns a string truncated to the given length
func TruncateString(s string, max int) string {
	if max > len(s) {
		return s
	}
	return s[:max]
}

// BoolToFloat64 converts a boolean to a float64. True is 1.0, false is 0.0
func BoolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
