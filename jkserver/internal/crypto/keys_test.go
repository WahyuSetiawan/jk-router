package crypto_test

import (
	"testing"

	"jkrouter/jkserver/internal/crypto"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plain := "sk-secret-api-key-12345"
	enc, err := crypto.EncryptGS([]byte(plain), key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == plain {
		t.Error("ciphertext should differ from plaintext")
	}
	dec, err := crypto.DecryptGS(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != plain {
		t.Errorf("mismatch: got %q, want %q", string(dec), plain)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i ^ 0xFF)
	}
	enc, _ := crypto.EncryptGS([]byte("secret"), key1)
	_, err := crypto.DecryptGS(enc, key2)
	if err == nil {
		t.Error("expected decrypt failure with wrong key")
	}
}
