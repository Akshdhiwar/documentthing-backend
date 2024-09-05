package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts the given plaintext using AES-GCM.
func Encrypt(plaintext, key []byte) (string, error) {
	// Create a new AES cipher block with the given key.
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Use GCM mode (Galois/Counter Mode) for AES.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create a nonce for GCM. It must be unique for each encryption.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt the data using the AES-GCM cipher.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return the base64-encoded ciphertext.
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts the given base64-encoded ciphertext using AES-GCM.
func Decrypt(ciphertextBase64 string, key []byte) (string, error) {
	// Decode the base64-encoded ciphertext.
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher block with the given key.
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Use GCM mode (Galois/Counter Mode) for AES.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// The first part of the ciphertext is the nonce.
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data using the AES-GCM cipher.
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	// Return the decrypted plaintext as a string.
	return string(plaintext), nil
}

// Example key derivation using SHA-256 (optional).
func DeriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}
