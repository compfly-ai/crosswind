package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewEncryptor(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "empty key returns error",
			key:     "",
			wantErr: true,
		},
		{
			name:    "valid base64 key",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 32)),
			wantErr: false,
		},
		{
			name:    "valid hex key",
			key:     hex.EncodeToString(make([]byte, 32)),
			wantErr: false,
		},
		{
			name:    "short key gets hashed",
			key:     "short-key",
			wantErr: false,
		},
		{
			name:    "long key gets truncated",
			key:     hex.EncodeToString(make([]byte, 64)),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncryptor(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if enc == nil {
				t.Error("expected encryptor, got nil")
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"simple text", "hello world"},
		{"special characters", "p@ssw0rd!#$%^&*()"},
		{"unicode", "こんにちは世界 🔐"},
		{"long text", strings.Repeat("a", 10000)},
		{"json payload", `{"api_key": "sk-123", "secret": "xyz"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			// Empty string returns empty
			if tt.plaintext == "" {
				if encrypted != "" {
					t.Errorf("expected empty, got %q", encrypted)
				}
				return
			}

			// Encrypted should have prefix
			if !strings.HasPrefix(encrypted, "encrypted:") {
				t.Errorf("expected 'encrypted:' prefix, got %q", encrypted)
			}

			// Decrypt should return original
			decrypted, err := enc.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("round-trip failed: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestDecryptWithoutPrefix(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewEncryptor(key)

	encrypted, _ := enc.Encrypt("test")
	// Remove prefix
	withoutPrefix := strings.TrimPrefix(encrypted, "encrypted:")

	decrypted, err := enc.Decrypt(withoutPrefix)
	if err != nil {
		t.Fatalf("decrypt without prefix failed: %v", err)
	}

	if decrypted != "test" {
		t.Errorf("got %q, want 'test'", decrypted)
	}
}

func TestDecryptInvalidData(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewEncryptor(key)

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"invalid base64", "not-valid-base64!!!"},
		{"too short", base64.StdEncoding.EncodeToString([]byte("x"))},
		{"wrong key data", base64.StdEncoding.EncodeToString(make([]byte, 100))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enc.Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestDifferentKeysCannotDecrypt(t *testing.T) {
	key1, _ := GenerateEncryptionKey()
	key2, _ := GenerateEncryptionKey()

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	encrypted, _ := enc1.Encrypt("secret data")

	_, err := enc2.Decrypt(encrypted)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestHashAPIKey(t *testing.T) {
	hash := HashAPIKey("test-api-key")

	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("expected 'sha256:' prefix, got %q", hash)
	}

	// SHA256 hex is 64 chars
	hexPart := strings.TrimPrefix(hash, "sha256:")
	if len(hexPart) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(hexPart))
	}

	// Same input should produce same hash
	hash2 := HashAPIKey("test-api-key")
	if hash != hash2 {
		t.Error("same input should produce same hash")
	}

	// Different input should produce different hash
	hash3 := HashAPIKey("different-key")
	if hash == hash3 {
		t.Error("different input should produce different hash")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key1, err := GenerateAPIKey("cw")
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if !strings.HasPrefix(key1, "cw_") {
		t.Errorf("expected 'cw_' prefix, got %q", key1)
	}

	// Should generate unique keys
	key2, _ := GenerateAPIKey("cw")
	if key1 == key2 {
		t.Error("generated keys should be unique")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Errorf("key should be valid base64: %v", err)
	}

	// Should be 32 bytes (256 bits)
	if len(decoded) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(decoded))
	}

	// Should be usable
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Errorf("generated key should be usable: %v", err)
	}
	if enc == nil {
		t.Error("encryptor should not be nil")
	}
}
