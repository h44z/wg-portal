package wireguard

import (
	"encoding/base64"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func GetPrivateKeyBytes(p persistence.KeyPair) []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PrivateKey)
	return data
}

func GetPublicKeyBytes(p persistence.KeyPair) []byte {
	data, _ := base64.StdEncoding.DecodeString(p.PublicKey)
	return data
}

func KeyBytesToString(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

type wgCtrlKeyGenerator struct{}

func (k wgCtrlKeyGenerator) GetFreshKeypair() (persistence.KeyPair, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return persistence.KeyPair{}, errors.Wrap(err, "failed to generate private Key")
	}

	return persistence.KeyPair{
		PrivateKey: privateKey.String(),
		PublicKey:  privateKey.PublicKey().String(),
	}, nil
}

func (k wgCtrlKeyGenerator) GetPreSharedKey() (persistence.PreSharedKey, error) {
	preSharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate pre-shared Key")
	}

	return persistence.PreSharedKey(preSharedKey.String()), nil
}
