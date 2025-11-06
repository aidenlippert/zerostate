package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

const (
	defaultKeystorePath = ".zerostate/keystore"
	privateKeyFile      = "identity.key"
)

// KeyStore manages persistent storage of identity keys
type KeyStore struct {
	path   string
	logger *zap.Logger
}

// NewKeyStore creates a new keystore at the specified path
func NewKeyStore(path string, logger *zap.Logger) *KeyStore {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			path = defaultKeystorePath
		} else {
			path = filepath.Join(home, defaultKeystorePath)
		}
	}

	return &KeyStore{
		path:   path,
		logger: logger,
	}
}

// LoadOrCreateSigner loads an existing identity or creates a new one
func (ks *KeyStore) LoadOrCreateSigner() (*Signer, error) {
	// Ensure keystore directory exists
	if err := os.MkdirAll(ks.path, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keystore directory: %w", err)
	}

	keyPath := filepath.Join(ks.path, privateKeyFile)

	// Try to load existing key
	if _, err := os.Stat(keyPath); err == nil {
		ks.logger.Info("loading existing identity", zap.String("path", keyPath))
		return ks.loadKey(keyPath)
	}

	// Create new key
	ks.logger.Info("creating new identity", zap.String("path", keyPath))
	return ks.createAndSaveKey(keyPath)
}

// loadKey loads a private key from disk
func (ks *KeyStore) loadKey(path string) (*Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Decode hex
	privateKeyBytes, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)
	
	// Use NewSignerFromKey which creates DID from the key
	signer, err := NewSignerFromKey(privateKey, ks.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	ks.logger.Info("identity loaded",
		zap.String("did", signer.DID()),
	)

	return signer, nil
}

// createAndSaveKey generates a new key pair and saves it
func (ks *KeyStore) createAndSaveKey(path string) (*Signer, error) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save to disk (hex encoded)
	keyHex := hex.EncodeToString(privateKey)
	if err := os.WriteFile(path, []byte(keyHex), 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Use NewSignerFromKey which creates DID from the key
	signer, err := NewSignerFromKey(privateKey, ks.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	ks.logger.Info("new identity created and saved",
		zap.String("did", signer.DID()),
		zap.String("path", path),
	)

	return signer, nil
}

// DeleteIdentity removes the stored identity (use with caution!)
func (ks *KeyStore) DeleteIdentity() error {
	keyPath := filepath.Join(ks.path, privateKeyFile)
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete identity: %w", err)
	}
	ks.logger.Warn("identity deleted", zap.String("path", keyPath))
	return nil
}
