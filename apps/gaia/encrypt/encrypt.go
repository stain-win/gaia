// Package encrypt provides encryption and decryption functionality for the Gaia daemon.
// It uses AES-256-GCM for symmetric encryption and scrypt for key derivation from passphrases.
// All encrypted data is base64-encoded for storage.
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	"golang.org/x/crypto/scrypt"
)

const (
	KeyLen = 32 // AES-256
)

// DeriveKey derives a key from the passphrase and salt using scrypt.
func DeriveKey(passphrase, salt []byte) ([]byte, error) {
	key, err := scrypt.Key(passphrase, salt, 1<<15, 8, 1, KeyLen)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("derive_key", "failed to derive key using scrypt", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
func Encrypt(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", gaiaerrors.NewCryptoError("encrypt", "failed to create AES cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", gaiaerrors.NewCryptoError("encrypt", "failed to create GCM cipher mode", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", gaiaerrors.NewCryptoError("encrypt", "failed to generate nonce", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
func Decrypt(key []byte, enc string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("decrypt", "failed to decode base64", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("decrypt", "failed to create AES cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("decrypt", "failed to create GCM cipher mode", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, gaiaerrors.NewCryptoError(
			"decrypt",
			fmt.Sprintf("ciphertext too short: length=%d, nonceSize=%d", len(ciphertext), gcm.NonceSize()),
			nil,
		)
	}
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("decrypt", "failed to decrypt data", err)
	}
	return plaintext, nil
}
