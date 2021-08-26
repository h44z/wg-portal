package wireguard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyPair_GetPrivateKeyBytes(t *testing.T) {
	kp := KeyPair{
		PrivateKey: "aGVsbG8=",
		PublicKey:  "d29ybGQ=",
	}

	got := kp.GetPrivateKeyBytes()
	assert.Equal(t, []byte("hello"), got)
}

func TestKeyPair_GetPublicKeyBytes(t *testing.T) {
	kp := KeyPair{
		PrivateKey: "aGVsbG8=",
		PublicKey:  "d29ybGQ=",
	}

	got := kp.GetPublicKeyBytes()
	assert.Equal(t, []byte("world"), got)
}

func TestKeyBytesToString(t *testing.T) {
	assert.Equal(t, "aGVsbG8=", KeyBytesToString([]byte("hello")))
}
