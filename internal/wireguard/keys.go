package wireguard

import "encoding/base64"

type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

type PreSharedKey string

func (p KeyPair) GetPrivateKeyBytes() []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PrivateKey)
	return data
}

func (p KeyPair) GetPublicKeyBytes() []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PublicKey)
	return data
}

func KeyBytesToString(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}
