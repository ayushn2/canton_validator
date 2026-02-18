package cantonvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

func (c *CantonGRPCClient) InstallWallet(
	ctx context.Context,
	validatorParty string,
	dsoParty string,
	endUserParty string,
	endUserName string,
	packageID string,
) error {
	payload := map[string]interface{}{
		"commands": map[string]interface{}{
			"workflow_id": "wf-" + endUserName,
			"user_id":     endUserName + "-user",
			"command_id":  "cmd-" + endUserName + "-1",
			"act_as": []string{
				endUserParty,
				validatorParty,
			},
			"commands": []interface{}{
				map[string]interface{}{
					"create": map[string]interface{}{
						"template_id": map[string]interface{}{
							"package_id":  packageID,
							"module_name": "Splice.Wallet.Install",
							"entity_name": "WalletAppInstall",
						},
						"create_arguments": map[string]interface{}{
							"fields": []interface{}{
								map[string]interface{}{
									"label": "validatorParty",
									"value": map[string]interface{}{
										"party": validatorParty,
									},
								},
								map[string]interface{}{
									"label": "dsoParty",
									"value": map[string]interface{}{
										"party": dsoParty,
									},
								},
								map[string]interface{}{
									"label": "endUserParty",
									"value": map[string]interface{}{
										"party": endUserParty,
									},
								},
								map[string]interface{}{
									"label": "endUserName",
									"value": map[string]interface{}{
										"text": endUserName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)

	fmt.Println("===== INSTALL WALLET PAYLOAD =====")
	fmt.Println(string(jsonBytes))
	fmt.Println("===================================")

	cmd := exec.Command(
		"grpcurl",
		"-plaintext",
		"-d", string(jsonBytes),
		cfg.GRPCHost,
		"com.daml.ledger.api.v2.CommandService/SubmitAndWait",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wallet install failed: %s", string(out))
	}

	fmt.Println("SubmitAndWait response:", string(out))
	return nil
}