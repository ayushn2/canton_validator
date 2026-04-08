package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ayushn2/canton_validator/cantonvalidator"
)

func main() {
	ctx := context.Background()

	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		log.Fatalf("failed to create canton client: %v", err)
	}
	defer client.Close()

	walletName := "ext-wallet-1"

	wallet, err := client.CreateExternalWallet(ctx, walletName)
	if err != nil {
		log.Fatalf("external wallet creation failed: %v", err)
	}

	fmt.Printf("\n✅ External wallet '%s' ready.\n", walletName)
	fmt.Printf("   Party ID      : %s\n", wallet.PartyID)
	fmt.Printf("   Public Key    : %s\n", wallet.PublicKeyHex)
	fmt.Printf("   ⚠️  Private key stored in wallets/wallets.json — encrypt before production.\n")
}
