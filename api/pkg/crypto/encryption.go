package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Encryptor handles encryption and decryption of sensitive data
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new encryptor with the given base64-encoded key
func NewEncryptor(encodedKey string) (*Encryptor, error) {
	if encodedKey == "" {
		return nil, fmt.Errorf("encryption key cannot be empty")
	}

	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		// Try using the key directly as hex
		key, err = hex.DecodeString(encodedKey)
		if err != nil {
			// Use SHA-256 hash of the key string
			hash := sha256.Sum256([]byte(encodedKey))
			key = hash[:]
		}
	}

	// Ensure key is 32 bytes for AES-256
	if len(key) < 32 {
		hash := sha256.Sum256(key)
		key = hash[:]
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return "encrypted:" + encoded, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *Encryptor) Decrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	// Remove the "encrypted:" prefix if present
	encrypted = strings.TrimPrefix(encrypted, "encrypted:")

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// HashAPIKey creates a SHA-256 hash of an API key
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// GenerateAPIKey generates a new random API key
func GenerateAPIKey(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// GenerateEncryptionKey generates a new random encryption key
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
