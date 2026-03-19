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

	// Scrypt parameters for key derivation
	// N = 2^17 = 131072 (recommended minimum for 2024+ security standards)
	// r = 8 (block size)
	// p = 1 (parallelization)
	// These parameters provide strong resistance against brute-force attacks.
	// WARNING: Changing these parameters will break existing databases!
	scryptN = 1 << 17
	scryptR = 8
	scryptP = 1
)

// DeriveKey derives a key from the passphrase and salt using scrypt.
// Uses secure parameters: N=2^17, r=8, p=1
func DeriveKey(passphrase, salt []byte) ([]byte, error) {
	key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, KeyLen)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("derive_key", "failed to derive key using scrypt", err)
	}
	return key, nil
}

// DeriveKeyLegacy derives a key using lighter scrypt parameters (N=2^15, ~4x faster).
// Use for: unsafe/dev mode databases and legacy migration. Never use for production.
func DeriveKeyLegacy(passphrase, salt []byte) ([]byte, error) {
	key, err := scrypt.Key(passphrase, salt, 1<<15, 8, 1, KeyLen)
	if err != nil {
		return nil, gaiaerrors.NewCryptoError("derive_key_legacy", "failed to derive key using scrypt", err)
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
