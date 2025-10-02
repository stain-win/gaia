package encrypt

import (
	"bytes"
	"testing"
)

func TestDeriveKey_Successful(t *testing.T) {
	passphrase := []byte("password")
	salt := []byte("salt")
	key, err := DeriveKey(passphrase, salt)
	if err != nil {
		t.Fatalf("DeriveKey() error = %v, wantErr nil", err)
	}
	if len(key) != 32 {
		t.Errorf("DeriveKey() key length = %d, want 32", len(key))
	}
}

func TestDeriveKey_Deterministic(t *testing.T) {
	passphrase := []byte("password")
	salt := []byte("salt")
	key1, _ := DeriveKey(passphrase, salt)
	key2, _ := DeriveKey(passphrase, salt)
	if !bytes.Equal(key1, key2) {
		t.Error("DeriveKey() produced different keys for the same input")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key, _ := DeriveKey([]byte("password"), []byte("salt"))
	plaintext := []byte("hello world")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypt() = %s, want %s", decrypted, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1, _ := DeriveKey([]byte("password"), []byte("salt"))
	key2, _ := DeriveKey([]byte("wrongpassword"), []byte("salt"))
	plaintext := []byte("hello world")

	ciphertext, _ := Encrypt(key1, plaintext)

	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("Decrypt() with wrong key should have failed, but it did not")
	}
}

func TestDecrypt_MalformedCiphertext(t *testing.T) {
	key, _ := DeriveKey([]byte("password"), []byte("salt"))
	_, err := Decrypt(key, "not-a-hex-string")
	if err == nil {
		t.Error("Decrypt() with malformed ciphertext should have failed, but it did not")
	}
}
