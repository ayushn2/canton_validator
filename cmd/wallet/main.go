package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ayushn2/canton_validator/cantonvalidator"
	"github.com/ayushn2/canton_validator/service"
)

func main() {
	ctx := context.Background()

	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		log.Fatalf("failed to create canton client: %v", err)
	}
	defer client.Close()

	walletName := "new-test-wallet-6"
	email := "new-test-wallet-6@scopex.money"
	password := "StrongPassword123!"

	svc := &service.WalletService{CantonGRPCClient: client}

	wallet, err := svc.CreateWallet(ctx, walletName, email, password)
	if err != nil {
		log.Fatalf("wallet creation failed: %v", err)
	}

	fmt.Printf("\n✅ Wallet '%s' ready.\n", walletName)
	fmt.Printf("   Auth0 User ID : %s\n", wallet.Auth0UserID)
	fmt.Printf("   Email         : %s\n", wallet.Email)
	fmt.Printf("   Canton User ID: %s\n", wallet.UserID)
	fmt.Printf("   Party ID      : %s::%s\n",
	strings.ReplaceAll(wallet.Auth0UserID, "|", "_"),
	strings.Split(wallet.PartyID, "::")[1],
	)
}