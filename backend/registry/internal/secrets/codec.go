// Package secrets encrypts secret values before they are persisted in etcd.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// keyVersion prefixes ciphertext so the master key can be rotated later.
const keyVersion byte = 1

// Codec encrypts and decrypts secret values with AES-256-GCM.
type Codec struct {
	aead cipher.AEAD
}

// New creates a codec from a raw 32-byte AES-256 key.
func New(key []byte) (*Codec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &Codec{aead: aead}, nil
}

// NewFromBase64 creates a codec from a base64-encoded 32-byte key.
func NewFromBase64(encoded string) (*Codec, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	return New(key)
}

// Encrypt returns base64(version || nonce || ciphertext) for plaintext.
func (c *Codec) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, 1+len(nonce)+len(sealed))
	payload = append(payload, keyVersion)
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt reverses Encrypt.
func (c *Codec) Decrypt(encoded string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(payload) < 1+c.aead.NonceSize() {
		return "", fmt.Errorf("ciphertext is too short")
	}
	if payload[0] != keyVersion {
		return "", fmt.Errorf("unsupported key version %d", payload[0])
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	ciphertext := payload[1+c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}
