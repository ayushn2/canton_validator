package cantonvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ayushn2/canton_validator/config"
)


type CantonGRPCClient struct{}

var cfg = config.Load()


func NewCantonGRPCClient() (*CantonGRPCClient, error) {
	return &CantonGRPCClient{}, nil
}

func (c *CantonGRPCClient) Close() {}

func (c *CantonGRPCClient) CreateParty(
	ctx context.Context,
	hint string,
) (string, error) {

	payload := map[string]interface{}{
		"party_id_hint": hint,
	}

	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.PartyManagementService/AllocateParty",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		output := string(out)

	if strings.Contains(output, "Party already exists") {

    fmt.Println("Party already exists. Constructing full party ID directly...")

    // Extract namespace from validator party
    // Format: scopex-validator-1::<namespace>
    parts := strings.Split("scopex-validator-1::12205f40f735c6d338ec14f0bcebe8de5c43f670ec9bb2666ede81806353a30a394c", "::")
    namespace := parts[1]

    fullParty := hint + "::" + namespace

    fmt.Println("Resolved full party ID:", fullParty)
    return fullParty, nil
}

		return "", fmt.Errorf("party creation failed: %s", output)
	}

	var resp struct {
		PartyDetails struct {
			Party string `json:"party"`
		} `json:"party_details"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("failed to parse party response: %w", err)
	}

	fmt.Println("New party allocated:", resp.PartyDetails.Party)

	return resp.PartyDetails.Party, nil
}

func (c *CantonGRPCClient) CreateUser(
	ctx context.Context,
	userID string,
) error {

	payload := map[string]interface{}{
		"user": map[string]interface{}{
			"id": userID,
		},
	}

	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.UserManagementService/CreateUser",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {

		// If user already exists, ignore
		if strings.Contains(string(out), "USER_ALREADY_EXISTS") ||
			strings.Contains(string(out), "already exists") {
			return nil
		}

		return fmt.Errorf("create user failed: %s", string(out))
	}

	return nil
}

func (c *CantonGRPCClient) GrantActAs(
	ctx context.Context,
	userID string,
	parties []string,
) error {

	rights := []map[string]interface{}{}

	for _, p := range parties {
		rights = append(rights, map[string]interface{}{
			"can_act_as": map[string]interface{}{
				"party": p,
			},
		})
	}

	payload := map[string]interface{}{
		"user_id": userID,
		"rights":  rights,
	}

	jsonBytes, _ := json.Marshal(payload)

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.admin.UserManagementService/GrantUserRights",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grant rights failed: %s", string(out))
	}

	return nil
}

func CreateWalletFullFlow() error {

	ctx := context.Background()

	grpcClient, err := NewCantonGRPCClient()
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	// 1. Create Party
	party, err := grpcClient.CreateParty(ctx, "walletX")
	if err != nil {
		return err
	}

	fmt.Println("Created party:", party)

	// 2. Install Wallet
	err = grpcClient.InstallWallet(
		ctx,
		"scopex-validator-1::12205f40f735c6d338ec14f0bcebe8de5c43f670ec9bb2666ede81806353a30a394c",
		"DSO::1220f22a8b8f2d813c25b9a684dc4dd52b532a0174d8e73a13cdf2baabfff7518337",
		party,
		"walletX",
		"fd57252dda29e3ce90028114c91b521cb661df5a9d6e87c41a9e91518215fa5b",
	)
	if err != nil {
		return err
	}

	fmt.Println("Wallet installed")

	return nil
}

func (c *CantonGRPCClient) WaitUntilPartyVisible(
	ctx context.Context,
	party string,
) error {

	for i := 0; i < 10; i++ {

		payload := map[string]interface{}{
			"parties": []string{party},
		}

		jsonBytes, _ := json.Marshal(payload)

		cmd := exec.Command(
			"grpcurl",
			"-plaintext",
			"-d", string(jsonBytes),
			cfg.GRPCHost,
			"com.daml.ledger.api.v2.admin.PartyManagementService/GetParties",
		)

		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), party) {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("party not visible after waiting")
}

