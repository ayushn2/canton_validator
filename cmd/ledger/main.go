package main

import (
	"context"
	"fmt"

	"github.com/ayushn2/canton_validator/cantonvalidator"
)

func main() {
	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	wallets, err := client.GetAllWallets(ctx)
	if err != nil {
		panic(err)
	}
	for _, wallet := range wallets {
		fmt.Printf("Username: %s, Party ID: %s\n", wallet.Username, wallet.PartyID)
	}
}
