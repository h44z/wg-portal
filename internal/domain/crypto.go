package domain

import (
	"encoding/base64"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type KeyPair struct {
	PrivateKey string `gorm:"serializer:encstr"`
	PublicKey  string
}

func (p KeyPair) GetPrivateKeyBytes() []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PrivateKey)
	return data
}

func (p KeyPair) GetPublicKeyBytes() []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PublicKey)
	return data
}

func (p KeyPair) GetPrivateKey() wgtypes.Key {
	key, _ := wgtypes.ParseKey(p.PrivateKey)
	return key
}

func (p KeyPair) GetPublicKey() wgtypes.Key {
	key, _ := wgtypes.ParseKey(p.PublicKey)
	return key
}

type PreSharedKey string

// NewFreshKeypair generates a new key pair.
func NewFreshKeypair() (KeyPair, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return KeyPair{}, err
	}

	return KeyPair{
		PrivateKey: privateKey.String(),
		PublicKey:  privateKey.PublicKey().String(),
	}, nil
}

// NewPreSharedKey generates a new pre-shared key.
func NewPreSharedKey() (PreSharedKey, error) {
	preSharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		return "", err
	}

	return PreSharedKey(preSharedKey.String()), nil
}

// PublicKeyFromPrivateKey returns the public key for a given private key.
// If the private key is invalid, an empty string is returned.
func PublicKeyFromPrivateKey(key string) string {
	privKey, err := wgtypes.ParseKey(key)
	if err != nil {
		return ""
	}
	return privKey.PublicKey().String()
}
