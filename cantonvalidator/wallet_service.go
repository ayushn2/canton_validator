package cantonvalidator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// CreateParty allocates a new party on the ledger using the party_id_hint.
// Returns the full party ID e.g. "walletC::1220..."
func (c *CantonGRPCClient) CreateParty(ctx context.Context, walletName string) (string, error) {
	payload := map[string]any{
		"party_id_hint": walletName,
	}
	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
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

// CreateUser creates a Daml user linked to the given party with act_as and read_as rights.
func (c *CantonGRPCClient) CreateUser(ctx context.Context, userID string, partyID string) error {
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

// OnboardWallet calls POST /api/validator/v0/register with a JWT for the given userID.
// This creates the WalletAppInstall contract on the ledger — completing onboarding.
// Returns the registered party ID.
func (c *CantonGRPCClient) OnboardWallet(ctx context.Context, userID string) (string, error) {
	// Generate JWT token for this user
	token, err := c.generateJWT(userID)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{})

	req, err := http.NewRequestWithContext(ctx, "POST",
		cfg.ValidatorURL+"/api/validator/v0/register",
		bytes.NewReader(reqBody),
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

// generateJWT creates a signed HS256 JWT for the given userID.
// Uses the same secret and audience as the validator node expects.
func (c *CantonGRPCClient) generateJWT(userID string) (string, error) {
	secret := cfg.JWTSecret   // e.g. "unsafe" for testnet
	audience := cfg.JWTAudience // e.g. "https://validator.example.com"

	now := time.Now().Unix()
	claims := map[string]any{
		"iat": now,
		"aud": audience,
		"sub": userID,
	}

	// Build header.payload
	headerJSON, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payloadJSON, _ := json.Marshal(claims)

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	msg := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return msg + "." + sig, nil
}

// CreateWallet orchestrates the full 3-step wallet creation and onboarding flow.
func (c *CantonGRPCClient) CreateWallet(
	ctx context.Context,
	walletName string,
) error {
	fmt.Println("===== CREATE WALLET FLOW START =====")

	// Step 1: Allocate party
	fmt.Println("Step 1: Allocating party:", walletName)
	partyID, err := c.CreateParty(ctx, walletName)
	if err != nil {
		return fmt.Errorf("create party failed: %w", err)
	}
	fmt.Println("Party allocated:", partyID)

	// Step 2: Create user
	userID := walletName + "-user"
	fmt.Println("Step 2: Creating user:", userID)
	if err := c.CreateUser(ctx, userID, partyID); err != nil {
		return fmt.Errorf("create user failed: %w", err)
	}
	fmt.Println("User created:", userID)

	// Step 3: Check if already onboarded (idempotency guard)
	fmt.Println("Step 3: Checking if wallet already installed...")
	exists, err := c.WalletAlreadyInstalled(ctx, "", partyID)
	if err != nil {
		return fmt.Errorf("wallet existence check failed: %w", err)
	}

	if exists {
		fmt.Println("Wallet already installed. Skipping onboarding.")
		fmt.Println("===== CREATE WALLET FLOW END =====")
		return nil
	}

	// Step 4: Onboard via validator API
	fmt.Println("Step 4: Onboarding wallet via validator API...")
	registeredParty, err := c.OnboardWallet(ctx, userID)
	if err != nil {
		return fmt.Errorf("onboard wallet failed: %w", err)
	}
	fmt.Println("Wallet onboarded successfully. Party:", registeredParty)

	fmt.Println("===== CREATE WALLET FLOW END =====")
	return nil
}

func (c *CantonGRPCClient) PreApproveTransfers(ctx context.Context, userID string) error {
    token, err := generateJWT(userID)
    if err != nil {
        return fmt.Errorf("failed to generate token: %w", err)
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

func generateJWT(userID string) (string, error) {
    // mirrors: jwt.encode({'iat': now, 'aud': '...', 'sub': userID}, 'unsafe', HS256)
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "iat": time.Now().Unix(),
        "aud": cfg.JWTAudience,
        "sub": userID,
    })
    return token.SignedString([]byte("unsafe"))
}

func (c *CantonGRPCClient) Transfer(
	ctx context.Context,
	senderUserID string,
	receiverPartyID string,
	amount string,
) error {
	token, err := generateJWT(senderUserID)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	payload := map[string]any{
		"receiver_party_id": receiverPartyID,
		"amount":            amount,
		"description":       "",
		"expires_at":        time.Now().UnixMicro() + 86400000000, // 24hrs
		"tracking_id":       uuid.New().String(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.ValidatorURL+"/api/validator/v0/wallet/token-standard/transfers",
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("transfer request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transfer failed (%d): %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("Transfer OK: %s\n", string(respBody))
	return nil
}

func (c *CantonGRPCClient) CreateAndSetupWallet(ctx context.Context, walletName string) (partyID, userID string, err error) {
    fmt.Println("===== CREATE AND SETUP WALLET FLOW START =====")

    // Step 1: Allocate party
    fmt.Printf("Allocating party for '%s'...\n", walletName)
    partyID, err = c.CreateParty(ctx, walletName)
    if err != nil {
        return "", "", fmt.Errorf("create party failed: %w", err)
    }
    fmt.Printf("✅ Party allocated: %s\n", partyID)

    // Step 2: Create user
    userID = walletName + "-user"
    fmt.Printf("Creating user '%s'...\n", userID)
    if err := c.CreateUser(ctx, userID, partyID); err != nil {
        return "", "", fmt.Errorf("create user failed: %w", err)
    }
    fmt.Printf("✅ User created: %s\n", userID)

    // Step 3: Wallet onboarding
    exists, err := c.WalletAlreadyInstalled(ctx, "", partyID)
    if err != nil {
        return "", "", fmt.Errorf("wallet existence check failed: %w", err)
    }

    if exists {
        fmt.Println("⚠️ Wallet already installed. Skipping onboarding.")
    } else {
        fmt.Printf("Onboarding wallet via validator API...\n")
        _, err := c.OnboardWallet(ctx, userID)
        if err != nil {
            return "", "", fmt.Errorf("onboard wallet failed: %w", err)
        }
        fmt.Println("✅ Wallet onboarded.")
    }

    // Step 4: Pre-approve transfers
    fmt.Printf("Pre-approving incoming transfers for '%s'...\n", userID)
    if err := c.PreApproveTransfers(ctx, userID); err != nil {
        return "", "", fmt.Errorf("transfer pre-approval failed: %w", err)
    }
    fmt.Println("✅ Transfer pre-approval active.")

    // Step 5: Optional verification
    fmt.Println("Fetching active contracts for verification...")
    contracts, err := c.GetActiveContracts(ctx, partyID, 0)
    if err != nil {
        return "", "", fmt.Errorf("failed to fetch active contracts: %w", err)
    }
    fmt.Println("Active Contracts:", contracts)

    fmt.Println("===== WALLET SETUP COMPLETE =====")
    return partyID, userID, nil
}