package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"slices"
)

// checkForPRNG is a function that checks if a cryptographically secure PRNG is available.
// If it is not available, the function panics.
func checkForPRNG() {
	buf := make([]byte, 1)
	_, err := io.ReadFull(rand.Reader, buf)

	if err != nil {
		panic(fmt.Sprintf("crypto/rand is unavailable: Read() failed with %#v", err))
	}
}

// generateToken is a function that generates a secure random CSRF token.
func generateToken(length int) []byte {
	bytes := make([]byte, length)

	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		panic(err)
	}

	return bytes
}

// encodeToken is a function that encodes a token to a base64 string.
func encodeToken(token []byte) string {
	return base64.URLEncoding.EncodeToString(token)
}

// decodeToken is a function that decodes a base64 string to a token.
func decodeToken(token string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(token)
}

// maskToken is a function that masks a token with a given key.
// The returned byte slice contains the key + the masked token.
// The key needs to have the same length as the token, otherwise the function panics.
// So the resulting slice has a length of len(token) * 2.
func maskToken(token, key []byte) []byte {
	if len(token) != len(key) {
		panic("token and key must have the same length")
	}

	// masked contains the key in the first half and the XOR masked token in the second half
	tokenLength := len(token)
	masked := make([]byte, tokenLength*2)
	for i := 0; i < len(token); i++ {
		masked[i] = key[i]
		masked[i+tokenLength] = token[i] ^ key[i] // XOR mask
	}

	return masked
}

// unmaskToken is a function that unmask a token which contains the key in the first half.
// The returned byte slice contains the unmasked token, it has exactly half the length of the input slice.
func unmaskToken(masked []byte) []byte {
	tokenLength := len(masked) / 2
	token := make([]byte, tokenLength)
	for i := 0; i < tokenLength; i++ {
		token[i] = masked[i] ^ masked[i+tokenLength] // XOR unmask
	}

	return token
}

// tokenEqual is a function that compares two tokens for equality.
func tokenEqual(a, b string) bool {
	decodedA, err := decodeToken(a)
	if err != nil {
		return false
	}
	decodedB, err := decodeToken(b)
	if err != nil {
		return false
	}

	unmaskedA := unmaskToken(decodedA)
	unmaskedB := unmaskToken(decodedB)

	return slices.Equal(unmaskedA, unmaskedB)
}
