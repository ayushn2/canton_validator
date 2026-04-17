package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ayushn2/canton_validator/cantonvalidator"
	"github.com/ayushn2/canton_validator/db"
	"github.com/ayushn2/canton_validator/service"
)

const (
	senderWallet   = "ayush-admin"
	receiverWallet = "new-test-wallet-4"
	amount         = "5.0"
)

func main() {
	ctx := context.Background()

	client, err := cantonvalidator.NewCantonGRPCClient()
	if err != nil {
		log.Fatalf("failed to create canton client: %v", err)
	}
	defer client.Close()

	store, err := db.LoadWalletStore()
	if err != nil {
		log.Fatalf("failed to load wallet store: %v", err)
	}

	sender, err := store.Get(senderWallet)
	if err != nil {
		log.Fatalf("sender not found in wallet store: %v", err)
	}

	receiver, err := store.Get(receiverWallet)
	if err != nil {
		log.Fatalf("receiver not found in wallet store: %v", err)
	}

	svc := &service.TransferService{CantonGRPCClient: client}

	fmt.Printf("Transferring %s CC from '%s' to '%s'...\n", amount, sender.Name, receiver.Name)

	if err := svc.Transfer(ctx, sender.Email, sender.Password, receiver.PartyID, amount); err != nil {
		log.Fatalf("transfer failed: %v", err)
	}

	fmt.Println("✅ Transfer complete.")
}