package secrets

import (
	"encoding/base64"
	"strings"
	"testing"
)

func newTestCodec(t *testing.T) *Codec {
	t.Helper()
	codec, err := New(make([]byte, 32))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return codec
}

func TestCodecRoundTrip(t *testing.T) {
	codec := newTestCodec(t)
	const plaintext = "correct horse battery staple"

	encrypted, err := codec.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := codec.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestCiphertextIsNotPlaintext(t *testing.T) {
	codec := newTestCodec(t)
	const plaintext = "super-secret-password"

	encrypted, err := codec.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("encrypted value equals the plaintext")
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("ciphertext is not base64: %v", err)
	}
	if strings.Contains(string(raw), plaintext) {
		t.Fatal("ciphertext contains the plaintext bytes")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	encryptor := newTestCodec(t)
	encrypted, err := encryptor.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	otherKey := make([]byte, 32)
	otherKey[0] = 1
	decryptor, err := New(otherKey)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := decryptor.Decrypt(encrypted); err == nil {
		t.Fatal("expected decryption with a different key to fail")
	}
}
