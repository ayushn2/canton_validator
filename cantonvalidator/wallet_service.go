package cantonvalidator

import (
	"context"
	"fmt"
)

func (c *CantonGRPCClient) CreateWallet(
	ctx context.Context,
	walletName string,
	validatorParty string,
	dsoParty string,
	packageID string,
) error {

	fmt.Println("===== CREATE WALLET FLOW START =====")

	// 1. Create Party
	fmt.Println("Creating party:", walletName)
	party, err := c.CreateParty(ctx, walletName)
	if err != nil {
		return fmt.Errorf("create party failed: %w", err)
	}
	fmt.Println("Resolved party:", party)

	// 2. Create User
	userID := walletName + "-user"
	fmt.Println("Creating user:", userID)
	err = c.CreateUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("create user failed: %w", err)
	}

	// 3. Wait until party is visible
	fmt.Println("Waiting for party propagation...")
	err = c.WaitUntilPartyVisible(ctx, party)
	if err != nil {
		return fmt.Errorf("party propagation failed: %w", err)
	}
	fmt.Println("Party visible.")

	// 4. Check if wallet already installed
	fmt.Println("Checking if wallet already installed...")
	exists, err := c.WalletAlreadyInstalled(ctx, packageID, party)
	if err != nil {
		return fmt.Errorf("wallet existence check failed: %w", err)
	}

	// 5. Grant rights
	fmt.Println("Granting act_as rights...")
	err = c.GrantActAs(
		ctx,
		userID,
		[]string{
			validatorParty,
			party,
		},
	)
	if err != nil {
		return fmt.Errorf("grant rights failed: %w", err)
	}

	// 6. Install wallet if not exists
	if exists {
		fmt.Println("Wallet already installed. Skipping install.")
	} else {
		fmt.Println("Installing wallet...")
		err = c.InstallWallet(
			ctx,
			validatorParty,
			dsoParty,
			party,
			walletName,
			packageID,
		)
		if err != nil {
			return fmt.Errorf("install wallet failed: %w", err)
		}
		fmt.Println("Wallet installed.")
	}

	fmt.Println("===== CREATE WALLET FLOW END =====")
	return nil
}