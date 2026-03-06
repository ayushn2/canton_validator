package config

import (
	"log"
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	// Canton connection
	ValidatorParty string
	DsoParty       string
	PackageID      string
	GRPCHost       string
	ValidatorURL   string

	// Auth — unsafe mode (current)
	JWTSecret   string
	JWTAudience string

	// Auth — Auth0 mode (future)
	Auth0Domain       string
	Auth0ClientID     string
	Auth0ClientSecret string
	Auth0Audience     string
	Auth0WalletUsername string
	Auth0WalletPassword string
	LedgerAPIAdminUser string
	TestWalletPassword string

	// Which auth mode to use
	AuthMode string // "unsafe" or "auth0"
}

func Load() *Config {
	err := godotenv.Load("config/.env")
	if err != nil {
		log.Println("Warning: .env file not loaded")
	}

	cfg := &Config{
    // Canton
    ValidatorParty: os.Getenv("VALIDATOR_PARTY"),
    DsoParty:       os.Getenv("DSO_PARTY"),
    PackageID:      os.Getenv("PACKAGE_ID"),
    GRPCHost:       os.Getenv("GRPC_HOST"),
    ValidatorURL:   os.Getenv("VALIDATOR_URL"),

    // Auth unsafe (current)
    JWTSecret:   getEnv("JWT_SECRET", "unsafe"),
    JWTAudience: getEnv("JWT_AUDIENCE", "https://validator.example.com"),

    // Auth0 — using SAME names as EC2 validator .env
    Auth0Domain:       os.Getenv("AUTH0_DOMAIN"),
    Auth0ClientID:     os.Getenv("VALIDATOR_AUTH_CLIENT_ID"),
    Auth0ClientSecret: os.Getenv("VALIDATOR_AUTH_CLIENT_SECRET"),
    Auth0Audience:     os.Getenv("VALIDATOR_AUTH_AUDIENCE"),

    // Mode
    AuthMode: getEnv("AUTH_MODE", "unsafe"),
	Auth0WalletUsername: os.Getenv("AUTH0_WALLET_USERNAME"),
	Auth0WalletPassword: os.Getenv("AUTH0_WALLET_PASSWORD"),
	LedgerAPIAdminUser: os.Getenv("LEDGER_API_ADMIN_USER"),
	TestWalletPassword: getEnv("TEST_WALLET_PASSWORD", "StrongPassword123!"),
}

	// Validate required fields
	if cfg.ValidatorParty == "" || cfg.DsoParty == "" ||
		cfg.PackageID == "" || cfg.ValidatorURL == "" {
		log.Fatal("Missing required environment variables")
	}

	return cfg
}

// getEnv returns the env var value or a default if empty
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}