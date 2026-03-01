package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ayushn2/canton_validator/cantonvalidator"
)

const (
	senderUserID  = "walletB-user"
	receiverPartyID = "test-wallet-7::12205f40f735c6d338ec14f0bcebe8de5c43f670ec9bb2666ede81806353a30a394c"
	amount        = 1.0
)

func main() {
	ctx := context.Background()

	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		log.Fatalf("failed to create canton client: %v", err)
	}
	defer client.Close()

	senderUserID  := senderUserID
	receiverPartyID := receiverPartyID
	amount        := amount

	fmt.Printf("Transferring %f CC from '%s' to '%s'...\n", amount, senderUserID, receiverPartyID)

	if err := client.Transfer(ctx, senderUserID, receiverPartyID, fmt.Sprintf("%f", amount)); err != nil {
		log.Fatalf("transfer failed: %v", err)
	}

	fmt.Println("✅ Transfer complete.")
}