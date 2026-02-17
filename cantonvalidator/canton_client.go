package cantonvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ValidatorBaseURL = "http://localhost:5003"
	AuthSecret       = "unsafe"
	AuthAudience     = "https://validator.example.com"
	WalletUser       = "administrator"
)

type CantonClient struct {
	httpClient *http.Client
	token      string
}

func NewCantonClient() (*CantonClient, error) {
	token, err := generateToken(WalletUser)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	return &CantonClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
	}, nil
}

func (c *CantonClient) Close() error {
	return nil
}

func generateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"aud": AuthAudience,
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(AuthSecret))
}

func (c *CantonClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, ValidatorBaseURL+path, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *CantonClient) ListTransactions(ctx context.Context, pageSize int) error {
	body := map[string]int{
		"page_size": pageSize,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/validator/v0/wallet/transactions", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("Transactions: %+v\n", result)
	return nil
}

func (c *CantonClient) GetBalance(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/validator/v0/wallet/balance", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("Balance: %+v\n", result)
	return nil
}

func (c *CantonClient) Transfer(ctx context.Context, receiver string, amount string) error {
	body := map[string]interface{}{
		"receiver": receiver,
		"amount":   amount,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/validator/v0/wallet/transfer", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("Transfer response: %+v\n", result)
	return nil
}
