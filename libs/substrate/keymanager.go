package substrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	agentcard "github.com/aidenlippert/zerostate/libs/agentcard-go"
)

// KeyManager handles agent keypair generation and secure storage
type KeyManager struct {
	encryptionKey []byte // 32-byte key for AES-256
}

// NewKeyManager creates a new key manager with the given encryption key
func NewKeyManager(encryptionSecret string) *KeyManager {
	// Derive a 32-byte key from the secret
	hash := sha256.Sum256([]byte(encryptionSecret))
	return &KeyManager{
		encryptionKey: hash[:],
	}
}

// GenerateAgentKeypair generates a new Ed25519 keypair for an agent
func (km *KeyManager) GenerateAgentKeypair() (publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, err error) {
	return agentcard.GenerateKeyPair()
}

// EncryptPrivateKey encrypts a private key using AES-256-GCM
func (km *KeyManager) EncryptPrivateKey(privateKey ed25519.PrivateKey) ([]byte, error) {
	block, err := aes.NewCipher(km.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt: nonce + ciphertext
	ciphertext := gcm.Seal(nonce, nonce, privateKey, nil)
	return ciphertext, nil
}

// DecryptPrivateKey decrypts a private key using AES-256-GCM
func (km *KeyManager) DecryptPrivateKey(encrypted []byte) (ed25519.PrivateKey, error) {
	block, err := aes.NewCipher(km.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	privateKey, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return ed25519.PrivateKey(privateKey), nil
}

// PublicKeyToHex converts a public key to hex string
func PublicKeyToHex(publicKey ed25519.PublicKey) string {
	return hex.EncodeToString(publicKey)
}

// PublicKeyFromHex converts a hex string to public key
func PublicKeyFromHex(hexStr string) (ed25519.PublicKey, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, want %d", len(bytes), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(bytes), nil
}
