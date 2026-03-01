package config

import (
	"log"
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	ValidatorParty string
	DsoParty       string
	PackageID      string
	GRPCHost       string
	JWTSecret	  string
	JWTAudience    string
	ValidatorURL   string
}

func Load() *Config {
	err := godotenv.Load("config/.env")
	if err != nil {
		log.Println("Warning: .env file not loaded")
	}

	cfg := &Config{
		ValidatorParty: os.Getenv("VALIDATOR_PARTY"),
		DsoParty:       os.Getenv("DSO_PARTY"),
		PackageID:      os.Getenv("PACKAGE_ID"),
		GRPCHost:       os.Getenv("GRPC_HOST"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTAudience:    os.Getenv("JWT_AUDIENCE"),
		ValidatorURL:   os.Getenv("VALIDATOR_URL"),
	}

	if cfg.ValidatorParty == "" || cfg.DsoParty == "" || cfg.PackageID == "" || cfg.JWTSecret == "" || cfg.JWTAudience == "" || cfg.ValidatorURL == "" {
		log.Fatal("Missing required environment variables")
	}

	return cfg
}