package main

import (
	"context"
	"fmt"
	"log"
	"github.com/ayushn2/canton_validator/cantonvalidator"
)

func main() {
	client, err := cantonvalidator.NewCantonClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Fetching transactions...")
	if err := client.ListTransactions(ctx, 20); err != nil {
		log.Fatalf("Failed to list transactions: %v", err)
	}

	fmt.Println("Fetching balance...")
	if err := client.GetBalance(ctx); err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
}