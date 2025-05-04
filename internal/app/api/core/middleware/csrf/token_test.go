package csrf

import (
	"encoding/base64"
	"testing"
)

func TestCheckForPRNG(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("checkForPRNG() panicked: %v", r)
		}
	}()
	checkForPRNG()
}

func TestGenerateToken(t *testing.T) {
	length := 32
	token := generateToken(length)
	if len(token) != length {
		t.Errorf("generateToken() returned token of length %d, expected %d", len(token), length)
	}
}

func TestEncodeToken(t *testing.T) {
	token := []byte("testtoken")
	encoded := encodeToken(token)
	expected := base64.URLEncoding.EncodeToString(token)
	if encoded != expected {
		t.Errorf("encodeToken() = %v, want %v", encoded, expected)
	}
}

func TestDecodeToken(t *testing.T) {
	token := "dGVzdHRva2Vu"
	expected := []byte("testtoken")
	decoded, err := decodeToken(token)
	if err != nil {
		t.Errorf("decodeToken() error = %v", err)
	}
	if string(decoded) != string(expected) {
		t.Errorf("decodeToken() = %v, want %v", decoded, expected)
	}
}

func TestMaskToken(t *testing.T) {
	token := []byte("testtoken")
	key := []byte("keykeykey")
	masked := maskToken(token, key)
	if len(masked) != len(token)*2 {
		t.Errorf("maskToken() returned masked token of length %d, expected %d", len(masked), len(token)*2)
	}
}

func TestUnmaskToken(t *testing.T) {
	token := []byte("testtoken")
	key := []byte("keykeykey")
	masked := maskToken(token, key)
	unmasked := unmaskToken(masked)
	if string(unmasked) != string(token) {
		t.Errorf("unmaskToken() = %v, want %v", unmasked, token)
	}
}

func TestTokenEqual(t *testing.T) {
	tokenA := encodeToken(maskToken([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}))
	tokenB := encodeToken(maskToken([]byte{0x01, 0x02, 0x03}, []byte{0x04, 0x05, 0x06}))
	if !tokenEqual(tokenA, tokenB) {
		t.Errorf("tokenEqual() = false, want true")
	}

	tokenC := encodeToken(maskToken([]byte{0x01, 0x02, 0x03}, []byte{0x07, 0x08, 0x09}))
	if !tokenEqual(tokenA, tokenC) {
		t.Errorf("tokenEqual() = false, want true")
	}

	tokenD := encodeToken(maskToken([]byte{0x09, 0x02, 0x03}, []byte{0x04, 0x05, 0x06}))
	if tokenEqual(tokenA, tokenD) {
		t.Errorf("tokenEqual() = true, want false")
	}
}
