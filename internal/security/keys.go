package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func GenerateSessionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate session key: %w", err)
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

func ValidateKeyLength(key string, expectedBytes int) error {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(key)
		if err != nil {
			return fmt.Errorf("key is not valid base64: %w", err)
		}
	}

	if len(decoded) != expectedBytes {
		return fmt.Errorf("key must be exactly %d bytes, got %d", expectedBytes, len(decoded))
	}

	return nil
}

func DecodeKey(key string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(key)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}
	}
	return decoded, nil
}
