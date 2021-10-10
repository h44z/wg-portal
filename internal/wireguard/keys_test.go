package wireguard

import (
	"testing"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/stretchr/testify/assert"
)

func TestGetPrivateKeyBytes(t *testing.T) {
	kp := persistence.KeyPair{
		PrivateKey: "aGVsbG8=",
		PublicKey:  "d29ybGQ=",
	}

	got := GetPrivateKeyBytes(kp)
	assert.Equal(t, []byte("hello"), got)
}

func TestGetPublicKeyBytes(t *testing.T) {
	kp := persistence.KeyPair{
		PrivateKey: "aGVsbG8=",
		PublicKey:  "d29ybGQ=",
	}

	got := GetPublicKeyBytes(kp)
	assert.Equal(t, []byte("world"), got)
}

func TestKeyBytesToString(t *testing.T) {
	assert.Equal(t, "aGVsbG8=", KeyBytesToString([]byte("hello")))
}

func TestWgCtrlKeyGenerator_GetFreshKeypair(t *testing.T) {
	m := wgCtrlKeyGenerator{}
	kp, err := m.GetFreshKeypair()
	assert.NoError(t, err)
	assert.NotEmpty(t, kp.PrivateKey)
	assert.NotEmpty(t, kp.PublicKey)
}

func TestWgCtrlKeyGenerator_GetPreSharedKey(t *testing.T) {
	m := wgCtrlKeyGenerator{}
	psk, err := m.GetPreSharedKey()
	assert.NoError(t, err)
	assert.NotEmpty(t, psk)
}
