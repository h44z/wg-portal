package common

import "fmt"

// mapDefaultString returns the string value for the given key or a default value
func mapDefaultString(m map[string]interface{}, key string, dflt string) string {
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

// uniqueStringSlice removes duplicates in the given string slice
func uniqueStringSlice(slice []string) []string {
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
