package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"golang.org/x/oauth2"

	"walmart-order-checker/internal/security"
)

type TokenStorage struct {
	db  *sql.DB
	key []byte
}

func NewTokenStorage(dbPath string) (*TokenStorage, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS oauth_tokens (
			email TEXT PRIMARY KEY,
			encrypted_token BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		environment := os.Getenv("ENVIRONMENT")
		if environment == "production" {
			return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required in production")
		}

		log.Println("WARNING: ENCRYPTION_KEY not set, generating temporary key (development only)")
		log.Println("WARNING: All encrypted data will be lost on restart!")
		var err error
		encryptionKey, err = security.GenerateEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("generate encryption key: %w", err)
		}
	}

	if err := security.ValidateKeyLength(encryptionKey, 32); err != nil {
		return nil, fmt.Errorf("invalid ENCRYPTION_KEY: %w", err)
	}

	keyBytes, err := security.DecodeKey(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

	return &TokenStorage{
		db:  db,
		key: keyBytes,
	}, nil
}

func (ts *TokenStorage) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ts.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (ts *TokenStorage) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ts.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (ts *TokenStorage) Save(email string, token *oauth2.Token) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	encrypted, err := ts.encrypt(tokenJSON)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	now := time.Now().Unix()
	_, err = ts.db.Exec(`
		INSERT INTO oauth_tokens (email, encrypted_token, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			encrypted_token = excluded.encrypted_token,
			updated_at = excluded.updated_at
	`, email, encrypted, now, now)

	if err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	return nil
}

func (ts *TokenStorage) Load(email string) (*oauth2.Token, error) {
	var encrypted []byte
	err := ts.db.QueryRow("SELECT encrypted_token FROM oauth_tokens WHERE email = ?", email).Scan(&encrypted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("query token: %w", err)
	}

	decrypted, err := ts.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(decrypted, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

func (ts *TokenStorage) Delete(email string) error {
	_, err := ts.db.Exec("DELETE FROM oauth_tokens WHERE email = ?", email)
	return err
}

func (ts *TokenStorage) ListEmails() ([]string, error) {
	rows, err := ts.db.Query("SELECT email FROM oauth_tokens ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	return emails, nil
}

func (ts *TokenStorage) Close() error {
	return ts.db.Close()
}
