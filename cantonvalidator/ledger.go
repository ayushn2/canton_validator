package cantonvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ayushn2/canton_validator/config"
)

func (c *CantonGRPCClient) GetActiveContracts(
	ctx context.Context,
	party string,
	offset int64,
) (string, error) {

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

	fmt.Println("===== GET ACTIVE CONTRACTS PAYLOAD =====")
	fmt.Println(payload)
	fmt.Println("========================================")

	cfg := config.Load()

	cmd := exec.CommandContext(
		ctx,
		"grpcurl",
		"-plaintext",
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

func (c *CantonGRPCClient) WalletAlreadyInstalled(
	ctx context.Context,
	packageID string,
	endUserParty string,
) (bool, error) {

	// 1️⃣ Get ledger end first
	ledgerCmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", `{}`,
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.StateService/GetLedgerEnd",
	)

	ledgerOut, err := ledgerCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get ledger end: %s", string(ledgerOut))
	}

	var ledgerResp struct {
		Offset string `json:"offset"`
	}

	if err := json.Unmarshal(ledgerOut, &ledgerResp); err != nil {
		return false, fmt.Errorf("failed to parse ledger end: %w", err)
	}

	if ledgerResp.Offset == "" {
		return false, fmt.Errorf("ledger end offset empty")
	}

	fmt.Println("Ledger end offset:", ledgerResp.Offset)

	// 2️⃣ Query active contracts at ledger end
	payload := map[string]interface{}{
		"active_at_offset": ledgerResp.Offset,
		"event_format": map[string]interface{}{
			"filters_by_party": map[string]interface{}{
				endUserParty: map[string]interface{}{
					"cumulative": map[string]interface{}{},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.StateService/GetActiveContracts",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("wallet existence check failed: %s", string(out))
	}

	outStr := string(out)

	// 3️⃣ Check for WalletAppInstall template specifically
	if strings.Contains(outStr, `"entityName": "WalletAppInstall"`) {
		fmt.Println("Wallet already installed.")
		return true, nil
	}

	fmt.Println("Wallet not installed.")
	return false, nil
}