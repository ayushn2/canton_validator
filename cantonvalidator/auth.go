package cantonvalidator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ayushn2/canton_validator/config"
	"github.com/golang-jwt/jwt/v5"
)

// ─────────────────────────────────────────────
// Token Cache — avoids hitting Auth0 every call
// ─────────────────────────────────────────────

type tokenCache struct {
	token     string
	expiresAt time.Time
	mu        sync.Mutex
}

var cache = &tokenCache{}

// ─────────────────────────────────────────────
// Auth0 response shape
// ─────────────────────────────────────────────

type auth0TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ─────────────────────────────────────────────
// GenerateToken — M2M entry point
// Used for admin/backend operations
// Switches between unsafe and Auth0 M2M
// ─────────────────────────────────────────────

func GenerateToken(cfg *config.Config, userID string) (string, error) {
	switch cfg.AuthMode {
	case "auth0":
		fmt.Println("[auth] mode: auth0 (Auth0 M2M client credentials)")
		return getAuth0TokenCached(cfg)
	default: // "unsafe"
		fmt.Printf("[auth] mode: unsafe (HS256 JWT) — user: %s\n", userID)
		return generateUnsafeJWT(cfg, userID)
	}
}

// ─────────────────────────────────────────────
// GetUserToken — User entry point
// Used for wallet operations (transfers etc.)
// Uses Resource Owner Password Grant
// ─────────────────────────────────────────────

func GetUserToken(cfg *config.Config, username string, password string) (string, error) {
	switch cfg.AuthMode {
	case "auth0":
		return FetchUserToken(cfg, username, password)
	default: // "unsafe"
		fmt.Printf("[auth] mode: unsafe (HS256 JWT) — user: %s\n", username)
		return generateUnsafeJWT(cfg, username)
	}
}

func FetchUserToken(cfg *config.Config, username string, password string) (string, error) {
	payload := map[string]string{
		"grant_type":    "password",
		"client_id":     cfg.Auth0ClientID,
		"client_secret": cfg.Auth0ClientSecret,
		"audience":      cfg.Auth0Audience,
		"username":      username,
		"password":      password,
		"scope":         "openid profile email",
		"connection":    "Username-Password-Authentication",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user token request: %w", err)
	}

	url := "https://" + cfg.Auth0Domain + "/oauth/token"

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("user token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read user token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth0 user token error (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp auth0TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse user token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("auth0 returned empty user token — check username, password and audience")
	}

	fmt.Printf("[auth] mode: auth0 user token — user: %s\n", username)
	return tokenResp.AccessToken, nil
}

// ─────────────────────────────────────────────
// Unsafe JWT
// Used for local devnet / docker-compose setups
// NEVER use in production
// ─────────────────────────────────────────────

func generateUnsafeJWT(cfg *config.Config, userID string) (string, error) {
	claims := jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
		"aud": cfg.JWTAudience,
		"sub": userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign unsafe JWT: %w", err)
	}

	return signed, nil
}

// ─────────────────────────────────────────────
// Auth0 — Client Credentials Flow (M2M)
// ─────────────────────────────────────────────

func getAuth0TokenCached(cfg *config.Config) (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.token != "" && time.Now().Before(cache.expiresAt.Add(-60*time.Second)) {
		return cache.token, nil
	}

	token, expiresIn, err := FetchAuth0Token(cfg)
	if err != nil {
		return "", err
	}

	cache.token = token
	cache.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return cache.token, nil
}

func FetchAuth0Token(cfg *config.Config) (string, int, error) {
	if cfg.Auth0Domain == "" || cfg.Auth0ClientID == "" || cfg.Auth0ClientSecret == "" || cfg.Auth0Audience == "" {
		return "", 0, fmt.Errorf(
			"auth0 config incomplete: ensure AUTH0_DOMAIN, VALIDATOR_AUTH_CLIENT_ID, VALIDATOR_AUTH_CLIENT_SECRET, VALIDATOR_AUTH_AUDIENCE are set",
		)
	}

	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     cfg.Auth0ClientID,
		"client_secret": cfg.Auth0ClientSecret,
		"audience":      cfg.Auth0Audience,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal auth0 payload: %w", err)
	}

	url := "https://" + cfg.Auth0Domain + "/oauth/token"

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("auth0 token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read auth0 response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("auth0 returned non-200 (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp auth0TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse auth0 token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("auth0 returned empty access token — check client_id, secret, and audience")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// ─────────────────────────────────────────────
// Auth0 Management API token fetch
// Uses management API audience instead of Canton audience
// ─────────────────────────────────────────────

func FetchAuth0ManagementToken(cfg *config.Config) (string, error) {
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     cfg.Auth0ClientID,
		"client_secret": cfg.Auth0ClientSecret,
		"audience":      "https://" + cfg.Auth0Domain + "/api/v2/",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal management token request: %w", err)
	}

	resp, err := http.Post(
		"https://"+cfg.Auth0Domain+"/oauth/token",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("management token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("management token error (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp auth0TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse management token: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// ─────────────────────────────────────────────
// createAuth0User via Management API
// ─────────────────────────────────────────────

type auth0CreateUserResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func CreateAuth0User(cfg *config.Config, mgmtToken string, email string, password string) (string, error) {
	payload := map[string]string{
		"email":      email,
		"password":   password,
		"connection": "Username-Password-Authentication",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal create user request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost,
		"https://"+cfg.Auth0Domain+"/api/v2/users",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build create user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mgmtToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create user request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create auth0 user failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var userResp auth0CreateUserResponse
	if err := json.Unmarshal(respBody, &userResp); err != nil {
		return "", fmt.Errorf("failed to parse create user response: %w", err)
	}

	return userResp.UserID, nil
}

// ─────────────────────────────────────────────
// InvalidateCache — force fresh Auth0 token
// call after rotating client secret
// ─────────────────────────────────────────────

func InvalidateCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.token = ""
	cache.expiresAt = time.Time{}
}