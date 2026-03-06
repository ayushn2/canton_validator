package cantonvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (c *CantonGRPCClient) Transfer(
	ctx context.Context,
	senderEmail string,
	senderPassword string,
	receiverPartyID string,
	amount string,
) error {
	token, err := GetUserToken(cfg, senderEmail, senderPassword)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
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