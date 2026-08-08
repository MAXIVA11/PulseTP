package pulsetp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

const (
	saltSize  = 16
	nonceSize = 12 // standard AES-GCM nonce size
	keySize   = 32 // AES-256
)

// Encrypt encrypts plaintext with a key derived from passphrase (via
// scrypt) and seals it with AES-256-GCM. The result is a single opaque
// blob (salt || nonce || ciphertext) meant to be used as the payload
// passed to Send: the wire format doesn't know or care that it's
// encrypted, it's just bytes like any other message.
//
// This is an application-layer add-on, not part of the core protocol:
// without it, PulseTP has no confidentiality at all, gap timing is public
// and the decode algorithm is public, so anyone who can observe the
// packets can decode the message exactly like the intended receiver.
func Encrypt(passphrase string, plaintext []byte) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("pulsetp: generate salt: %w", err)
	}
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("pulsetp: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, saltSize+nonceSize+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt reverses Encrypt. A wrong passphrase or any corruption/truncation
// in transit fails GCM authentication and returns an error, it never
// silently returns garbage plaintext.
func Decrypt(passphrase string, data []byte) ([]byte, error) {
	if len(data) < saltSize+nonceSize {
		return nil, errors.New("pulsetp: payload too short to be encrypted (wrong key, or sender didn't encrypt)")
	}
	salt := data[:saltSize]
	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("pulsetp: could not decrypt (wrong key, or corrupted/truncated data)")
	}
	return plaintext, nil
}

func deriveKey(passphrase string, salt []byte) ([]byte, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, 1<<15, 8, 1, keySize)
	if err != nil {
		return nil, fmt.Errorf("pulsetp: derive key: %w", err)
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("pulsetp: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("pulsetp: create gcm: %w", err)
	}
	return gcm, nil
}
