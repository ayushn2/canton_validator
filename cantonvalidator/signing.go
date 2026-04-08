package cantonvalidator

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// WalletKeyPair holds an Ed25519 key pair for an external party wallet.
// Keys are hex-encoded to match the validator API's expected format.
type WalletKeyPair struct {
	PublicKeyHex  string // 32 bytes = 64 hex chars
	PrivateKeyHex string // 64 bytes = 128 hex chars — store securely, never log
}

// GenerateWalletKeyPair creates a fresh Ed25519 key pair.
func GenerateWalletKeyPair() (*WalletKeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	return &WalletKeyPair{
		PublicKeyHex:  hex.EncodeToString(pub),
		PrivateKeyHex: hex.EncodeToString(priv),
	}, nil
}

// SignHashHex signs a hex-encoded hash with the Ed25519 private key.
// Returns a hex-encoded signature in ${r}${s} form (64 bytes = 128 hex chars)
// as required by topology/submit and setup-proposal/submit-accept.
func (kp *WalletKeyPair) SignHashHex(hashHex string) (string, error) {
	privBytes, err := hex.DecodeString(kp.PrivateKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode hash: %w", err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(privBytes), hashBytes)
	return hex.EncodeToString(sig), nil
}
