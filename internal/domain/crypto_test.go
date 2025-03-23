package domain

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestKeyPair_GetPrivateKeyBytesReturnsCorrectBytes(t *testing.T) {
	keyPair := KeyPair{PrivateKey: base64.StdEncoding.EncodeToString([]byte("privateKey"))}
	expected := []byte("privateKey")
	assert.Equal(t, expected, keyPair.GetPrivateKeyBytes())
}

func TestKeyPair_GetPublicKeyBytesReturnsCorrectBytes(t *testing.T) {
	keyPair := KeyPair{PublicKey: base64.StdEncoding.EncodeToString([]byte("publicKey"))}
	expected := []byte("publicKey")
	assert.Equal(t, expected, keyPair.GetPublicKeyBytes())
}

func TestKeyPair_GetPrivateKeyReturnsCorrectKey(t *testing.T) {
	privateKey, _ := wgtypes.GeneratePrivateKey()
	keyPair := KeyPair{PrivateKey: privateKey.String()}
	assert.Equal(t, privateKey, keyPair.GetPrivateKey())
}

func TestKeyPair_GetPublicKeyReturnsCorrectKey(t *testing.T) {
	privateKey, _ := wgtypes.GeneratePrivateKey()
	keyPair := KeyPair{PublicKey: privateKey.PublicKey().String()}
	assert.Equal(t, privateKey.PublicKey(), keyPair.GetPublicKey())
}

func TestNewFreshKeypairGeneratesValidKeypair(t *testing.T) {
	keyPair, err := NewFreshKeypair()
	assert.NoError(t, err)
	assert.NotEmpty(t, keyPair.PrivateKey)
	assert.NotEmpty(t, keyPair.PublicKey)
}

func TestNewPreSharedKeyGeneratesValidKey(t *testing.T) {
	preSharedKey, err := NewPreSharedKey()
	assert.NoError(t, err)
	assert.NotEmpty(t, preSharedKey)
}

func TestPublicKeyFromPrivateKeyReturnsCorrectPublicKey(t *testing.T) {
	privateKey, _ := wgtypes.GeneratePrivateKey()
	expected := privateKey.PublicKey().String()
	assert.Equal(t, expected, PublicKeyFromPrivateKey(privateKey.String()))
}

func TestPublicKeyFromPrivateKeyReturnsEmptyStringOnInvalidKey(t *testing.T) {
	assert.Equal(t, "", PublicKeyFromPrivateKey("invalidKey"))
}
