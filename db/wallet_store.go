package db

import (
	"encoding/json"
	"fmt"
	"os"
)

const walletStoreFile = "wallets/wallets.json" // root/wallets/wallets.json

// WalletEntry holds all credentials and IDs for a wallet
type WalletEntry struct {
	Name         string `json:"name"`          // e.g. "walletB"
	Email        string `json:"email"`         // Auth0 email (standard flow only)
	Password     string `json:"password"`      // Auth0 password (standard flow only)
	Auth0UserID  string `json:"auth0_user_id"` // e.g. auth0|xxxxxxxxx (standard flow only)
	CantonUserID string `json:"canton_user_id"` // e.g. walletB-user (standard flow only)
	PartyID      string `json:"party_id"`      // full Canton party ID
	PublicKeyHex  string `json:"public_key_hex,omitempty"`  // hex ed25519 public key (external party flow)
	PrivateKeyHex string `json:"private_key_hex,omitempty"` // hex ed25519 private key (external party flow) ⚠️ encrypt in prod
}

// WalletStore holds all wallets
type WalletStore struct {
	Wallets []WalletEntry `json:"wallets"`
}

// LoadWalletStore loads the wallet store from disk
func LoadWalletStore() (*WalletStore, error) {
	data, err := os.ReadFile(walletStoreFile)
	if os.IsNotExist(err) {
		return &WalletStore{Wallets: []WalletEntry{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet store: %w", err)
	}

	var store WalletStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse wallet store: %w", err)
	}
	return &store, nil
}

// Save persists the wallet store to disk
func (s *WalletStore) Save() error {
	if err := os.MkdirAll("wallets", 0700); err != nil {
		return fmt.Errorf("failed to create wallets dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal wallet store: %w", err)
	}

	if err := os.WriteFile(walletStoreFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write wallet store: %w", err)
	}
	return nil
}

// Add adds a wallet entry to the store
func (s *WalletStore) Add(entry WalletEntry) {
	// Replace if already exists
	for i, w := range s.Wallets {
		if w.Name == entry.Name {
			s.Wallets[i] = entry
			return
		}
	}
	s.Wallets = append(s.Wallets, entry)
}

// Get returns a wallet by name
func (s *WalletStore) Get(name string) (*WalletEntry, error) {
	for _, w := range s.Wallets {
		if w.Name == name {
			return &w, nil
		}
	}
	return nil, fmt.Errorf("wallet '%s' not found in store", name)
}

// List prints all wallets in the store
func (s *WalletStore) List() {
	if len(s.Wallets) == 0 {
		fmt.Println("No wallets in store.")
		return
	}
	fmt.Printf("%-20s %-35s %-20s\n", "NAME", "EMAIL", "CANTON USER ID")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	for _, w := range s.Wallets {
		fmt.Printf("%-20s %-35s %-20s\n", w.Name, w.Email, w.CantonUserID)
	}
}