package cantonvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ayushn2/canton_validator/db"
)

// ─────────────────────────────────────────────────────────────────
// topology/generate types
// ─────────────────────────────────────────────────────────────────

type topologyTx struct {
	TopologyTx string `json:"topology_tx"` // base64-encoded topology transaction
	Hash       string `json:"hash"`        // hex-encoded hash to sign
}

type generateTopologyResponse struct {
	PartyID     string       `json:"party_id"`
	TopologyTxs []topologyTx `json:"topology_txs"`
}

// ─────────────────────────────────────────────────────────────────
// topology/submit types
// ─────────────────────────────────────────────────────────────────

type signedTopologyTx struct {
	TopologyTx string `json:"topology_tx"` // base64 (unchanged from generate)
	SignedHash string `json:"signed_hash"` // hex-encoded ed25519 sig: ${r}${s}
}

// ─────────────────────────────────────────────────────────────────
// setup-proposal/prepare-accept response
// ─────────────────────────────────────────────────────────────────

type prepareAcceptResponse struct {
	Transaction string `json:"transaction"` // base64-encoded PreparedTransaction protobuf
	TxHash      string `json:"tx_hash"`     // hex-encoded hash to sign
}

// ─────────────────────────────────────────────────────────────────
// GenerateExternalPartyTopology
// POST /api/validator/v0/admin/external-party/topology/generate
//
// Creates three topology transactions:
//   - root namespace tx (creates the party + sets namespace public key)
//   - party-to-participant mapping tx (hosts party with Confirmation rights)
//   - party-to-key mapping tx (sets the key for Daml transaction auth)
//
// Returns the generated party_id and the list of topology_txs to sign.
// Each TopologyTx carries its own hash — sign each one independently.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) GenerateExternalPartyTopology(
	ctx context.Context,
	adminToken string,
	partyHint string,
	pubKeyHex string,
) (*generateTopologyResponse, error) {

	payload := map[string]any{
		"party_hint": partyHint,
		"public_key": pubKeyHex,
	}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/admin/external-party/topology/generate",
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("build topology/generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("topology/generate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("topology/generate returned %d: %s", resp.StatusCode, body)
	}

	var result generateTopologyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse topology/generate response: %w", err)
	}
	if len(result.TopologyTxs) == 0 {
		return nil, fmt.Errorf("topology/generate returned no topology transactions")
	}
	return &result, nil
}

// ─────────────────────────────────────────────────────────────────
// SubmitExternalPartyTopology
// POST /api/validator/v0/admin/external-party/topology/submit
//
// Constructs a SignedTopologyTransaction and writes it to the
// authorized store. Pass each topology_tx unchanged from generate
// plus its signed_hash (hex r||s ed25519 signature over the hash).
//
// Returns the final party_id confirmed on the ledger.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) SubmitExternalPartyTopology(
	ctx context.Context,
	adminToken string,
	pubKeyHex string,
	signedTxs []signedTopologyTx,
) (string, error) {

	payload := map[string]any{
		"public_key":          pubKeyHex,
		"signed_topology_txs": signedTxs,
	}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/admin/external-party/topology/submit",
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return "", fmt.Errorf("build topology/submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("topology/submit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("topology/submit returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		PartyID string `json:"party_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse topology/submit response: %w", err)
	}
	if result.PartyID == "" {
		return "", fmt.Errorf("topology/submit returned empty party_id")
	}
	return result.PartyID, nil
}

// ─────────────────────────────────────────────────────────────────
// CreateExternalPartySetupProposal
// POST /api/validator/v0/admin/external-party/setup-proposal
//
// Validator operator creates an ExternalPartySetupProposal contract.
// The external party must then accept it via prepare-accept + submit-accept.
//
// Returns the contract_id of the proposal.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) CreateExternalPartySetupProposal(
	ctx context.Context,
	adminToken string,
	partyID string,
) (string, error) {

	payload := map[string]any{"user_party_id": partyID}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/admin/external-party/setup-proposal",
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return "", fmt.Errorf("build setup-proposal request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("setup-proposal request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("setup-proposal returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		ContractID string `json:"contract_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse setup-proposal response: %w", err)
	}
	if result.ContractID == "" {
		return "", fmt.Errorf("setup-proposal returned empty contract_id")
	}
	return result.ContractID, nil
}

// ─────────────────────────────────────────────────────────────────
// PrepareAcceptSetupProposal
// POST /api/validator/v0/admin/external-party/setup-proposal/prepare-accept
//
// Prepares the acceptance transaction so it can be signed externally.
// Returns the base64 PreparedTransaction and the hex tx_hash to sign.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) PrepareAcceptSetupProposal(
	ctx context.Context,
	adminToken string,
	contractID string,
	partyID string,
) (*prepareAcceptResponse, error) {

	payload := map[string]any{
		"contract_id":   contractID,
		"user_party_id": partyID,
	}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/admin/external-party/setup-proposal/prepare-accept",
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("build prepare-accept request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prepare-accept request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prepare-accept returned %d: %s", resp.StatusCode, body)
	}

	var result prepareAcceptResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse prepare-accept response: %w", err)
	}
	if result.TxHash == "" {
		return nil, fmt.Errorf("prepare-accept returned empty tx_hash")
	}
	return &result, nil
}

// ─────────────────────────────────────────────────────────────────
// SubmitAcceptSetupProposal
// POST /api/validator/v0/admin/external-party/setup-proposal/submit-accept
//
// Submits the signed acceptance of the ExternalPartySetupProposal.
// Activates transfer preapproval for the external party without
// creating a ValidatorRight — does NOT count toward the 200-wallet cap.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) SubmitAcceptSetupProposal(
	ctx context.Context,
	adminToken string,
	partyID string,
	pubKeyHex string,
	transaction string,
	signedTxHash string,
) error {

	payload := map[string]any{
		"submission": map[string]any{
			"party_id":       partyID,
			"transaction":    transaction,
			"signed_tx_hash": signedTxHash,
			"public_key":     pubKeyHex,
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/admin/external-party/setup-proposal/submit-accept",
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return fmt.Errorf("build submit-accept request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("submit-accept request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("submit-accept returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────
// ExternalWalletUser holds the result of a CreateExternalWallet call
// ─────────────────────────────────────────────────────────────────

type ExternalWalletUser struct {
	PartyID       string
	PublicKeyHex  string
	PrivateKeyHex string // ⚠️ store securely — never log
}

// ─────────────────────────────────────────────────────────────────
// CreateExternalWallet — scalable flow (no 200-wallet cap):
// 1. Generate Ed25519 key pair
// 2. topology/generate  → get topology txs + party_id
// 3. Sign each tx hash with Ed25519 key
// 4. topology/submit    → confirm party on ledger
// 5. setup-proposal     → operator creates proposal contract
// 6. prepare-accept     → get PreparedTransaction + tx_hash to sign
// 7. Sign tx_hash with Ed25519 key
// 8. submit-accept      → activate transfer preapproval
//
// Does NOT call /register or /wallet/transfer-preapproval.
// Does NOT create WalletAppInstall or ValidatorRight contracts.
// ─────────────────────────────────────────────────────────────────

func (c *CantonGRPCClient) CreateExternalWallet(
	ctx context.Context,
	walletName string,
) (*ExternalWalletUser, error) {
	fmt.Println("===== CREATE EXTERNAL WALLET FLOW START =====")

	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	// Step 1: Generate Ed25519 key pair
	fmt.Println("Step 1: Generating Ed25519 key pair...")
	keyPair, err := GenerateWalletKeyPair()
	if err != nil {
		return nil, fmt.Errorf("key generation failed: %w", err)
	}
	fmt.Printf("✅ Public key: %s\n", keyPair.PublicKeyHex)

	// Step 2: Generate topology transactions
	fmt.Printf("Step 2: Generating topology transactions for party hint: %s\n", walletName)
	topoResp, err := c.GenerateExternalPartyTopology(ctx, adminToken, walletName, keyPair.PublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("topology/generate failed: %w", err)
	}
	fmt.Printf("✅ Got %d topology tx(s), party_id: %s\n", len(topoResp.TopologyTxs), topoResp.PartyID)

	// Step 3: Sign each topology tx hash
	fmt.Println("Step 3: Signing topology tx hashes...")
	signedTxs := make([]signedTopologyTx, len(topoResp.TopologyTxs))
	for i, tx := range topoResp.TopologyTxs {
		sig, err := keyPair.SignHashHex(tx.Hash)
		if err != nil {
			return nil, fmt.Errorf("signing topology tx %d failed: %w", i, err)
		}
		signedTxs[i] = signedTopologyTx{
			TopologyTx: tx.TopologyTx,
			SignedHash: sig,
		}
	}
	fmt.Println("✅ All topology txs signed.")

	// Step 4: Submit signed topology
	fmt.Println("Step 4: Submitting signed topology transactions...")
	partyID, err := c.SubmitExternalPartyTopology(ctx, adminToken, keyPair.PublicKeyHex, signedTxs)
	if err != nil {
		return nil, fmt.Errorf("topology/submit failed: %w", err)
	}
	fmt.Printf("✅ External party registered on ledger: %s\n", partyID)

	// Step 5: Operator creates setup proposal
	fmt.Println("Step 5: Creating external party setup proposal...")
	contractID, err := c.CreateExternalPartySetupProposal(ctx, adminToken, partyID)
	if err != nil {
		return nil, fmt.Errorf("setup-proposal failed: %w", err)
	}
	fmt.Printf("✅ Setup proposal created: %s\n", contractID)

	// Step 6: Prepare acceptance transaction
	fmt.Println("Step 6: Preparing acceptance transaction...")
	prepResp, err := c.PrepareAcceptSetupProposal(ctx, adminToken, contractID, partyID)
	if err != nil {
		return nil, fmt.Errorf("prepare-accept failed: %w", err)
	}
	fmt.Println("✅ Acceptance transaction prepared.")

	// Step 7: Sign the tx hash
	fmt.Println("Step 7: Signing acceptance tx hash...")
	signedTxHash, err := keyPair.SignHashHex(prepResp.TxHash)
	if err != nil {
		return nil, fmt.Errorf("signing tx hash failed: %w", err)
	}
	fmt.Println("✅ Acceptance tx hash signed.")

	// Step 8: Submit signed acceptance
	fmt.Println("Step 8: Submitting signed acceptance...")
	if err := c.SubmitAcceptSetupProposal(
		ctx, adminToken, partyID, keyPair.PublicKeyHex,
		prepResp.Transaction, signedTxHash,
	); err != nil {
		return nil, fmt.Errorf("submit-accept failed: %w", err)
	}
	fmt.Println("✅ Transfer preapproval active.")

	fmt.Println("===== CREATE EXTERNAL WALLET FLOW END =====")

	// Save to wallet store
	store, err := db.LoadWalletStore()
	if err != nil {
		fmt.Printf("⚠️  Warning: could not load wallet store: %v\n", err)
	} else {
		store.Add(db.WalletEntry{
			Name:          walletName,
			PartyID:       partyID,
			PublicKeyHex:  keyPair.PublicKeyHex,
			PrivateKeyHex: keyPair.PrivateKeyHex,
		})
		if err := store.Save(); err != nil {
			fmt.Printf("⚠️  Warning: could not save wallet store: %v\n", err)
		} else {
			fmt.Println("✅ Wallet saved to store.")
		}
	}

	return &ExternalWalletUser{
		PartyID:       partyID,
		PublicKeyHex:  keyPair.PublicKeyHex,
		PrivateKeyHex: keyPair.PrivateKeyHex,
	}, nil
}
