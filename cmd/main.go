package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ayushn2/canton_validator/cantonvalidator"
	"github.com/ayushn2/canton_validator/config"
)

func main() {
	ctx := context.Background()

	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		log.Fatalf("failed to create canton client: %v", err)
	}
	defer client.Close()

	// ---------------- Wallet ----------------
	fmt.Println("Creating Wallet Z...")

	cfg := config.Load()

	err = client.CreateWallet(
		ctx,
		"walletX",
		cfg.ValidatorParty,
		cfg.DsoParty,
		cfg.PackageID,
	)
	if err != nil {
		log.Fatalf("wallet creation failed: %v", err)
	}

	fmt.Println("Wallet X created successfully")
	fmt.Println("All wallets created successfully.")

	contracts, err := client.GetActiveContracts(
	ctx,
	"walletX::12205f40f735c6d338ec14f0bcebe8de5c43f670ec9bb2666ede81806353a30a394c",
	45760, // or fetch ledger end dynamically
	)
	if err != nil {
		log.Fatalf("failed to fetch active contracts: %v", err)
	}

	fmt.Println("Active Contracts:")
	fmt.Println(contracts)
}