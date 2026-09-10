package cors

import "strings"

// wildcard is a type that represents a wildcard string.
// This type allows faster matching of strings with a wildcard
// in comparison to using regex.
type wildcard struct {
	prefix string
	suffix string
}

// match returns true if the string s has the prefix and suffix of the wildcard.
func (w wildcard) match(s string) bool {
	return len(s) >= len(w.prefix)+len(w.suffix) &&
		strings.HasPrefix(s, w.prefix) &&
		strings.HasSuffix(s, w.suffix)
}

func newWildcard(s string) wildcard {
	if before, after, ok := strings.Cut(s, "*"); ok {
		return wildcard{
			prefix: before,
			suffix: after,
		}
	}

	// fallback, usually this case should not happen
	return wildcard{
		prefix: s,
		suffix: "",
	}
}
