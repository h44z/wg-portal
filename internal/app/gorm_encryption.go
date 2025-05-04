package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm/schema"

	"github.com/h44z/wg-portal/internal/domain"
)

// GormEncryptedStringSerializer is a GORM serializer that encrypts and decrypts string values using AES256.
// It is used to store sensitive information in the database securely.
// If the serializer encounters a value that is not a string, it will return an error.
type GormEncryptedStringSerializer struct {
	useEncryption bool
	keyPhrase     string
	prefix        string
}

// NewGormEncryptedStringSerializer creates a new GormEncryptedStringSerializer.
// It needs to be registered with GORM to be used:
// schema.RegisterSerializer("encstr", gormEncryptedStringSerializerInstance)
// You can then use it in your model like this:
//
//	EncryptedField string `gorm:"serializer:encstr"`
func NewGormEncryptedStringSerializer(keyPhrase string) GormEncryptedStringSerializer {
	return GormEncryptedStringSerializer{
		useEncryption: keyPhrase != "",
		keyPhrase:     keyPhrase,
		prefix:        "WG_ENC_",
	}
}

// Scan implements the GORM serializer interface. It decrypts the value after reading it from the database.
func (s GormEncryptedStringSerializer) Scan(
	ctx context.Context,
	field *schema.Field,
	dst reflect.Value,
	dbValue any,
) (err error) {
	var dbStringValue string
	if dbValue != nil {
		switch v := dbValue.(type) {
		case []byte:
			dbStringValue = string(v)
		case string:
			dbStringValue = v
		default:
			return fmt.Errorf("unsupported type %T for encrypted field %s", dbValue, field.Name)
		}
	}

	if !s.useEncryption {
		field.ReflectValueOf(ctx, dst).SetString(dbStringValue) // keep the original value
		return nil
	}

	if !strings.HasPrefix(dbStringValue, s.prefix) {
		field.ReflectValueOf(ctx, dst).SetString(dbStringValue) // keep the original value
		return nil
	}

	encryptedString := strings.TrimPrefix(dbStringValue, s.prefix)
	decryptedString, err := DecryptAES256(encryptedString, s.keyPhrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt value for field %s: %w", field.Name, err)
	}

	field.ReflectValueOf(ctx, dst).SetString(decryptedString)
	return
}

// Value implements the GORM serializer interface. It encrypts the value before storing it in the database.
func (s GormEncryptedStringSerializer) Value(
	_ context.Context,
	_ *schema.Field,
	_ reflect.Value,
	fieldValue any,
) (any, error) {
	if fieldValue == nil {
		return nil, nil
	}

	switch v := fieldValue.(type) {
	case string:
		if v == "" {
			return "", nil // empty string, no need to encrypt
		}
		if !s.useEncryption {
			return v, nil // keep the original value
		}
		encryptedString, err := EncryptAES256(v, s.keyPhrase)
		if err != nil {
			return nil, err
		}
		return s.prefix + encryptedString, nil
	case domain.PreSharedKey:
		if v == "" {
			return "", nil // empty string, no need to encrypt
		}
		if !s.useEncryption {
			return string(v), nil // keep the original value
		}
		encryptedString, err := EncryptAES256(string(v), s.keyPhrase)
		if err != nil {
			return nil, err
		}
		return s.prefix + encryptedString, nil
	default:
		return nil, fmt.Errorf("encryption only supports string values, got %T", fieldValue)
	}
}

// EncryptAES256 encrypts the given plaintext with the given key using AES256 in CBC mode with PKCS7 padding
func EncryptAES256(plaintext, key string) (string, error) {
	if len(plaintext) == 0 {
		return "", fmt.Errorf("plaintext must not be empty")
	}
	if len(key) == 0 {
		return "", fmt.Errorf("key must not be empty")
	}
	key = trimEncKey(key)
	iv := key[:aes.BlockSize]

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	plain := []byte(plaintext)
	plain = pkcs7Padding(plain, aes.BlockSize)

	ciphertext := make([]byte, len(plain))

	mode := cipher.NewCBCEncrypter(block, []byte(iv))
	mode.CryptBlocks(ciphertext, plain)

	b64String := base64.StdEncoding.EncodeToString(ciphertext)

	return b64String, nil
}

// DecryptAES256 decrypts the given ciphertext with the given key using AES256 in CBC mode with PKCS7 padding
func DecryptAES256(encrypted, key string) (string, error) {
	if len(encrypted) == 0 {
		return "", fmt.Errorf("ciphertext must not be empty")
	}
	if len(key) == 0 {
		return "", fmt.Errorf("key must not be empty")
	}
	key = trimEncKey(key)
	iv := key[:aes.BlockSize]

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length, must be a multiple of %d", aes.BlockSize)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	mode := cipher.NewCBCDecrypter(block, []byte(iv))
	mode.CryptBlocks(ciphertext, ciphertext)

	ciphertext = pkcs7UnPadding(ciphertext)

	return string(ciphertext), nil
}

func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func pkcs7UnPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

func trimEncKey(key string) string {
	if len(key) > 32 {
		return key[:32]
	}

	if len(key) < 32 {
		key = key + strings.Repeat("0", 32-len(key))
	}
	return key
}
