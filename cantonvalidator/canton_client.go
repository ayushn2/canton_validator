package cantonvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ayushn2/canton_validator/config"
)


type CantonGRPCClient struct{}

var cfg = config.Load()


func NewCantonGRPCClient() (*CantonGRPCClient, error) {
	return &CantonGRPCClient{}, nil
}

func (c *CantonGRPCClient) Close() {}

func (c *CantonGRPCClient) GrantActAs(
	ctx context.Context,
	userID string,
	parties []string,
) error {

	rights := []map[string]any{}

	for _, p := range parties {
		rights = append(rights, map[string]any{
			"can_act_as": map[string]any{
				"party": p,
			},
		})
	}

	payload := map[string]any{
		"user_id": userID,
		"rights":  rights,
	}

	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.UserManagementService/GrantUserRights",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grant rights failed: %s", string(out))
	}

	return nil
}

func (c *CantonGRPCClient) WaitUntilPartyVisible(
	ctx context.Context,
	party string,
) error {

	for range 10 {

		payload := map[string]any{
			"parties": []string{party},
		}

		jsonBytes, _ := json.Marshal(payload)

		cmd := exec.Command(
			"grpcurl",
			"-plaintext",
			"-d", string(jsonBytes),
			cfg.GRPCHost,
			"com.daml.ledger.api.v2.admin.PartyManagementService/GetParties",
		)

		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), party) {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("party not visible after waiting")
}
