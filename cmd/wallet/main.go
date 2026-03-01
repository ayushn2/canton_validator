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

    walletName := "test-wallet-8"
    partyID, userID, err := client.CreateAndSetupWallet(ctx, walletName)
    if err != nil {
        log.Fatalf("wallet setup failed: %v", err)
    }

    fmt.Printf("Wallet '%s' ready. User ID: %s, Party ID: %s\n", walletName, userID, partyID)
}