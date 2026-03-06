package cantonvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func (c *CantonGRPCClient) GetActiveContracts(
	ctx context.Context,
	party string,
	offset int64,
) (string, error) {
	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}

	payload := fmt.Sprintf(`{
	  "active_at_offset": %d,
	  "event_format": {
	    "filters_by_party": {
	      "%s": {
	        "cumulative": {}
	      }
	    }
	  }
	}`, offset, party)

	cmd := exec.CommandContext(
		ctx,
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", payload,
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.StateService/GetActiveContracts",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("GetActiveContracts failed: %s", string(out))
	}

	return string(out), nil
}

// WalletAlreadyInstalled checks if a WalletAppInstall contract exists for the given party.
func (c *CantonGRPCClient) WalletAlreadyInstalled(
	ctx context.Context,
	packageID string,
	endUserParty string,
) (bool, error) {
	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return false, fmt.Errorf("failed to get admin token: %w", err)
	}

	// Get ledger end
	ledgerCmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", `{}`,
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.StateService/GetLedgerEnd",
	)
	ledgerOut, err := ledgerCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get ledger end: %s", string(ledgerOut))
	}

	var ledgerResp struct {
		Offset json.Number `json:"offset"`
	}
	if err := json.Unmarshal(ledgerOut, &ledgerResp); err != nil {
		return false, fmt.Errorf("failed to parse ledger end: %w", err)
	}
	if ledgerResp.Offset == "" {
		return false, fmt.Errorf("ledger end offset empty")
	}

	offsetInt, err := ledgerResp.Offset.Int64()
	if err != nil {
		return false, fmt.Errorf("failed to convert offset to int: %w", err)
	}

	// Query ACS
	payload := map[string]interface{}{
		"active_at_offset": offsetInt,
		"event_format": map[string]interface{}{
			"filters_by_party": map[string]interface{}{
				endUserParty: map[string]interface{}{
					"cumulative": []map[string]interface{}{
						{
							"wildcard_filter": map[string]interface{}{
								"include_created_event_blob": false,
							},
						},
					},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.StateService/GetActiveContracts",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("wallet existence check failed: %s", string(out))
	}

	if strings.Contains(string(out), `"entity_name": "WalletAppInstall"`) {
		return true, nil
	}
	return false, nil
}

// WalletInfo holds username and partyID for a wallet user.
type WalletInfo struct {
	Username string `json:"username"`
	PartyID  string `json:"party_id"`
}

// GetAllWallets returns all users and their party IDs from the ledger.
func (c *CantonGRPCClient) GetAllWallets(ctx context.Context) ([]WalletInfo, error) {
	adminToken, err := GenerateToken(cfg, cfg.LedgerAPIAdminUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		"grpcurl",
		"-plaintext",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", `{}`,
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.UserManagementService/ListUsers",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ListUsers failed: %s", string(out))
	}

	var resp struct {
		Users []struct {
			Id           string `json:"id"`
			PrimaryParty string `json:"primary_party"`
		} `json:"users"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse ListUsers output: %w", err)
	}

	wallets := make([]WalletInfo, 0, len(resp.Users))
	for _, user := range resp.Users {
		wallets = append(wallets, WalletInfo{
			Username: user.Id,
			PartyID:  user.PrimaryParty,
		})
	}
	return wallets, nil
}