package cantonvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"github.com/ayushn2/canton_validator/db"
)

// Use topology-specific config for wallet operations, which may differ from the main GRPC client config and help scale the number of wallets created

// WalletUser holds the result of a successful CreateWallet call
type WalletUser struct {
	Auth0UserID string // e.g. auth0|xxxxxxxxx
	Email       string
	PartyID     string // Canton party ID after onboarding
	UserID      string // Canton user ID e.g. walletName-user
}

// ─────────────────────────────────────────────
// CreateParty allocates a new party on the ledger
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) CreateParty(ctx context.Context, walletName string) (string, error) {
	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}

	payload := map[string]any{
		"party_id_hint": walletName,
	}
	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.PartyManagementService/AllocateParty",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("AllocateParty failed: %s", string(out))
	}

	var resp struct {
		PartyDetails struct {
			Party string `json:"party"`
		} `json:"party_details"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("failed to parse AllocateParty response: %w", err)
	}
	if resp.PartyDetails.Party == "" {
		return "", fmt.Errorf("party ID empty in AllocateParty response")
	}

	return resp.PartyDetails.Party, nil
}

// ─────────────────────────────────────────────
// CreateUser creates a Daml user linked to a party
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) CreateUser(ctx context.Context, userID string, partyID string) error {
	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	payload := map[string]any{
		"user": map[string]any{
			"id":            userID,
			"primary_party": partyID,
		},
		"rights": []map[string]any{
			{"can_act_as": map[string]any{"party": partyID}},
			{"can_read_as": map[string]any{"party": partyID}},
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.UserManagementService/CreateUser",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("CreateUser failed: %s", string(out))
	}

	var resp struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("failed to parse CreateUser response: %w", err)
	}
	if resp.User.ID == "" {
		return fmt.Errorf("user ID empty in CreateUser response")
	}

	return nil
}

// ─────────────────────────────────────────────
// OnboardWallet calls /register to create
// WalletAppInstall contract on the ledger
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) OnboardWallet(ctx context.Context, email string, password string) (string, error) {
	token, err := GetUserToken(cfg, email, password)
	if err != nil {
		return "", fmt.Errorf("failed to get user token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/register",
		bytes.NewReader([]byte("{}")),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("register returned %d: %s", resp.StatusCode, string(body))
	}

	var registerResp struct {
		PartyID string `json:"party_id"`
	}
	if err := json.Unmarshal(body, &registerResp); err != nil {
		return "", fmt.Errorf("failed to parse register response: %w", err)
	}
	if registerResp.PartyID == "" {
		return "", fmt.Errorf("party_id empty in register response")
	}

	return registerResp.PartyID, nil
}

// ─────────────────────────────────────────────
// PreApproveTransfers enables auto-accept
// for incoming transfers
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) PreApproveTransfers(ctx context.Context, email string, password string) error {
	token, err := GetUserToken(cfg, email, password)
	if err != nil {
		return fmt.Errorf("failed to get user token: %w", err)
	}

	body := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/wallet/transfer-preapproval", body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict { // 409 = already exists
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pre-approval failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ─────────────────────────────────────────────
// CreateWallet — full flow:
// 1. Create Auth0 user
// 2. Allocate Canton party
// 3. Create Canton user
// 4. Onboard wallet via /register
// 5. Pre-approve transfers
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) CreateWallet(
	ctx context.Context,
	walletName string,
	email string,
	password string,
) (*WalletUser, error) {
	fmt.Println("===== CREATE WALLET FLOW START =====")

	// Step 1: Create Auth0 user
	fmt.Printf("Step 1: Creating Auth0 user: %s\n", email)
	mgmtToken, err := fetchAuth0ManagementToken(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get management token: %w", err)
	}
	auth0UserID, err := createAuth0User(cfg, mgmtToken, email, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth0 user: %w", err)
	}
	fmt.Printf("✅ Auth0 user created: %s (%s)\n", email, auth0UserID)

	// Step 2: Allocate Canton party
	fmt.Printf("Step 2: Allocating Canton party: %s\n", walletName)
	partyID, err := c.CreateParty(ctx, walletName)
	if err != nil {
		return nil, fmt.Errorf("create party failed: %w", err)
	}
	fmt.Printf("✅ Party allocated: %s\n", partyID)

	// Step 3: Create Canton user
	userID := walletName + "-user"
	fmt.Printf("Step 3: Creating Canton user: %s\n", userID)
	if err := c.CreateUser(ctx, userID, partyID); err != nil {
		return nil, fmt.Errorf("create canton user failed: %w", err)
	}
	fmt.Printf("✅ Canton user created: %s\n", userID)

	// Step 4: Onboard wallet
	fmt.Println("Step 4: Onboarding wallet via validator API...")
	registeredPartyID, err := c.OnboardWallet(ctx, email, password)
	if err != nil {
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "already") {
			fmt.Println("⚠️ Wallet already installed. Skipping onboarding.")
		} else {
			return nil, fmt.Errorf("onboard wallet failed: %w", err)
		}
	} else {
		partyID = registeredPartyID // real party ID from /register
		fmt.Println("✅ Wallet onboarded.")
	}

	// Step 5: Pre-approve transfers
	fmt.Println("Step 5: Pre-approving incoming transfers...")
	if err := c.PreApproveTransfers(ctx, email, password); err != nil {
		return nil, fmt.Errorf("transfer pre-approval failed: %w", err)
	}
	fmt.Println("✅ Transfer pre-approval active.")

	fmt.Println("===== CREATE WALLET FLOW END =====")

	// Save to wallet store
	store, err := db.LoadWalletStore()
	if err != nil {
		fmt.Printf("⚠️  Warning: could not load wallet store: %v\n", err)
	} else {
		store.Add(db.WalletEntry{
			Name:         walletName,
			Email:        email,
			Password:     password,
			Auth0UserID:  auth0UserID,
			CantonUserID: userID,
			PartyID:      partyID,
		})
		if err := store.Save(); err != nil {
			fmt.Printf("⚠️  Warning: could not save wallet store: %v\n", err)
		} else {
			fmt.Println("✅ Wallet saved to store.")
		}
	}

	return &WalletUser{
		Auth0UserID: auth0UserID,
		Email:       email,
		PartyID:     partyID,
		UserID:      userID,
	}, nil
}

// ─────────────────────────────────────────────
// CreateAndSetupWallet — legacy wrapper
// kept for backward compatibility
// ─────────────────────────────────────────────

func (c *CantonGRPCClient) CreateAndSetupWallet(ctx context.Context, walletName string, email string, password string) (partyID, userID string, err error) {
	wallet, err := c.CreateWallet(ctx, walletName, email, password)
	if err != nil {
		return "", "", err
	}
	return wallet.PartyID, wallet.UserID, nil
}