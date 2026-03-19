# USDCx Integration Guide — Canton Validator

## ⚠️ Prerequisites — Read Before Starting

**The validator node must be fully onboarded and healthy before starting USDCx integration.** This means:

- Validator is running and all containers show `(healthy)`
- Auth0 (or equivalent OIDC provider) is configured with M2M client credentials
- Nginx is configured with SSL and all API endpoints are exposed
- Dfns BYOV integration is complete (if applicable)
- You can successfully generate a JWT token and hit `/api/validator/v0/version`

Do NOT proceed with USDCx integration until all of the above are confirmed.

---

## Overview

Circle and Digital Asset have partnered to bring **USDC onto the Canton Network as USDCx** — a CIP-56 Token Standard compliant stablecoin. Users lock real USDC on Ethereum via Circle's `xReserve` contract, which mints USDCx on Canton. Burning USDCx on Canton releases USDC back on Ethereum.

### Three Operations

| Operation | Trigger | Frequency |
| --- | --- | --- |
| **Onboard** | Register party with xReserve bridge | Once per party |
| **Mint** | User deposits USDC on Ethereum | On demand |
| **Burn** | User withdraws USDCx to Ethereum | On demand |

---

## Environment Variables

The only values that differ between testnet and mainnet are the party IDs and backend URL. Everything else — commands, DAR names, API paths — is identical.

### Testnet
```
UTILITY_BACKEND_URL=https://api.utilities.digitalasset-staging.com
ADMIN_PARTY_ID=decentralized-usdc-interchain-rep::122049e2af8a725bd19759320fc83c638e7718973eac189d8f201309c512d1ffec61
UTILITY_OPERATOR_PARTY_ID=DigitalAsset-UtilityOperator::12202679f2bbe57d8cba9ef3cee847ac8239df0877105ab1f01a77d47477fdce1204
BRIDGE_OPERATOR_PARTY_ID=Bridge-Operator::12209d011ce250de439fefc35d16d1ab9d56fb99ccb24c18d798efb22352d533bcdb
```

### Mainnet
```
UTILITY_BACKEND_URL=https://api.utilities.digitalasset.com
ADMIN_PARTY_ID=decentralized-usdc-interchain-rep::12208115f1e168dd7e792320be9c4ca720c751a02a3053c7606e1c1cd3dad9bf60ef
UTILITY_OPERATOR_PARTY_ID=auth0_007c6643538f2eadd3e573dd05b9::12205bcc106efa0eaa7f18dc491e5c6f5fb9b0cc68dc110ae66f4ed6467475d7c78e
BRIDGE_OPERATOR_PARTY_ID=Bridge-Operator::1220c8448890a70e65f6906bd48d797ee6551f094e9e6a53e329fd5b2b549334f13f
```

### Your validator (fill in)
```
VALIDATOR_URL=https://<your-validator-domain>
LEDGER_URL=https://<your-ledger-api-domain>
TOKEN=<your-jwt-token>
```

Throughout this guide, replace `<VALIDATOR_URL>`, `<LEDGER_URL>`, and `<TOKEN>` with your values. All party IDs above are fixed — do not change them.

---

## Step 1 — Verify Validator Version

USDCx requires validator version `0.5.12` or later. Check your version:

```bash
curl -s https://<VALIDATOR_URL>/api/validator/version \
  -H "Authorization: Bearer $TOKEN"
# Expected: {"version":"0.5.12",...} or higher
```

If below `0.5.12`, upgrade before proceeding. Refer to your validator operator docs for upgrade steps.

---

## Step 2 — Add USDCx Environment Variables to .env

Add the following to your validator's `.env` file. Use testnet or mainnet values from the table above.

```bash
cat >> .env << 'EOF'

# USDCx / CUB config
CUB_IMAGE_REPO="europe-docker.pkg.dev/da-images/public/docker"
CUB_IMAGE_VERSION="0.3.0"
CROSS_CHAIN_REPRESENTATIVE_PARTY_ID="<ADMIN_PARTY_ID>"
UTILITY_BACKEND_URL="<UTILITY_BACKEND_URL>"
EOF
```

Verify:

```bash
tail -6 .env
```

---

## Step 3 — Add cub-darsyncer to compose.yaml

The `cub-darsyncer` automatically uploads the `utility-bridge-v0` and `utility-bridge-app-v0` DARs to your participant node.

Open your `compose.yaml` and add the following service block inside the `services:` section, just before your `nginx` service:

```yaml
  cub-darsyncer:
    image: "${CUB_IMAGE_REPO}/cub-darsyncer-client:${CUB_IMAGE_VERSION}"
    command:
      - --endpoint=participant:5002
    environment:
      - DARS=/dars
      - CLIENT_ID=<your-m2m-client-id>
      - CLIENT_SECRET=<your-m2m-client-secret>
      - OAUTH_DOMAIN=<your-auth-domain>
    depends_on:
      - participant
      - validator
    networks:
      - <your-docker-network>
```

> Replace `CLIENT_ID`, `CLIENT_SECRET`, and `OAUTH_DOMAIN` with your M2M credentials. These are the same credentials your validator uses to authenticate with the ledger API. Replace `<your-docker-network>` with the network name used by your other services.

Validate, pull, and start:

```bash
# Validate compose file
docker compose config --quiet && echo "✅ valid" || echo "❌ errors"

# Pull the image
docker compose pull cub-darsyncer

# Start darsyncer only (no need to restart everything)
docker compose up -d cub-darsyncer

# Watch logs — confirm both DARs uploaded successfully
docker logs -f <cub-darsyncer-container-name> 2>&1 | head -50
```

Expected output:
```
successfully uploaded: utility-bridge-app-v0-0.1.3.dar
successfully uploaded: utility-bridge-v0-0.1.3.dar
the ledger has all of our packages, as expected
```

---

## Step 4 — Fix nginx Upload Size Limit

Large DAR files exceed nginx's default body size limit and will return `HTTP 413`. Add this to the `http {}` block in your `nginx.conf`:

```nginx
http {
    client_max_body_size 50m;
    ...
}
```

Reload nginx without restarting:

```bash
docker exec <nginx-container-name> nginx -s reload
```

---

## Step 5 — Download and Upload Utility Registry DARs

The `utility-registry-app-v0` DAR is required for USDCx transactions at runtime. It is not included in the darsyncer bundle and must be uploaded manually.

### Find the correct bundle version

Call the utility backend to get the exact package hash your network requires:

```bash
curl -s -X POST ${UTILITY_BACKEND_URL}/api/utilities/v0/registry/burn-mint-instruction/v0/burn-mint-factory \
  -H "Content-Type: application/json" \
  -d "{
    \"instrumentId\": {
      \"admin\": \"${ADMIN_PARTY_ID}\",
      \"id\": \"USDCx\"
    },
    \"inputHoldingCids\": [],
    \"outputs\": []
  }" | python3 -c "
import sys, json
data = json.load(sys.stdin)
template = data['choiceContext']['disclosedContracts'][0]['templateId']
pkg_hash = template.split(':')[0]
print('Exact package hash needed:', pkg_hash)
"
```

Cross-reference this hash against the DAR versions page to find the correct bundle version:
https://docs.digitalasset.com/utilities/devnet/reference/dar-versions/dar-versions.html

### Download the bundle

```bash
# Replace <VERSION> with the bundle version matching your hash
wget https://get.digitalasset.com/utility-dars/canton-network-utility-dars-<VERSION>.tar.gz

mkdir -p utility-dars-<VERSION>
tar -xzf canton-network-utility-dars-<VERSION>.tar.gz -C utility-dars-<VERSION>/

# Confirm registry DARs are present
ls utility-dars-<VERSION>/ | grep -i "registry"
```

### Upload registry DARs in dependency order

```bash
for dar in \
  utility-dars-<VERSION>/utility-registry-v0-*.dar \
  utility-dars-<VERSION>/utility-registry-holding-v0-*.dar \
  utility-dars-<VERSION>/utility-registry-app-v0-*.dar; do
  echo "Uploading $dar..."
  HTTP=$(curl -s -o /tmp/dar_response.json -w "%{http_code}" \
    -X POST https://<LEDGER_URL>/v2/packages \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @"$dar")
  echo "HTTP: $HTTP"
  cat /tmp/dar_response.json
  echo ""
done
```

### Verify the target hash is present

```bash
curl -s https://<LEDGER_URL>/v2/packages \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
data = json.load(sys.stdin)
pkgs = data.get('packageIds', [])
target = '<PACKAGE_HASH_FROM_FACTORY>'
print('Total packages:', len(pkgs))
print('utility-registry present:', target in pkgs)
if target in pkgs:
    print('✅ Ready for USDCx onboarding!')
else:
    print('❌ Still missing — try a different bundle version')
"
```

---

## Step 6 — Submit BridgeUserAgreementRequest (Onboarding)

This must be done **once per party** that needs USDCx access. Repeat for every wallet/party you create.

```bash
USER_PARTY_ID="<party-id-to-onboard>"

curl -s -w "\nHTTP:%{http_code}" \
  -X POST https://<LEDGER_URL>/v2/commands/submit-and-wait-for-transaction \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"commands\": {
      \"commandId\": \"usdcx-onboard-$(date +%s)\",
      \"actAs\": [\"$USER_PARTY_ID\"],
      \"readAs\": [\"$USER_PARTY_ID\"],
      \"commands\": [
        {
          \"CreateCommand\": {
            \"templateId\": \"#utility-bridge-v0:Utility.Bridge.V0.Agreement.User:BridgeUserAgreementRequest\",
            \"createArguments\": {
              \"crossChainRepresentative\": \"${ADMIN_PARTY_ID}\",
              \"operator\": \"${UTILITY_OPERATOR_PARTY_ID}\",
              \"bridgeOperator\": \"${BRIDGE_OPERATOR_PARTY_ID}\",
              \"user\": \"$USER_PARTY_ID\",
              \"instrumentId\": {
                \"admin\": \"${ADMIN_PARTY_ID}\",
                \"id\": \"USDCx\"
              },
              \"preApproval\": false
            }
          }
        }
      ]
    }
  }"
```

Expected: `HTTP:200` with a `transactionId` and a `BridgeUserAgreementRequest` contract in the events.

---

## Step 7 — Poll for Bridge Operator Approval

After submitting the request, Circle's bridge operator must approve it. On testnet this is automated. On mainnet it may take longer.

```bash
curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [
            {
              \"wildcardFilter\": {
                \"includeCreatedEventBlob\": false
              }
            }
          ]
        }
      }
    },
    \"verbose\": false
  }" | python3 -c "
import sys
raw = sys.stdin.read()
if 'BridgeUserAgreement' in raw:
    print('✅ Approved — party is fully onboarded to xReserve')
else:
    print('⏳ Pending approval — try again in a few minutes')
"
```

Once approved, the party has a `BridgeUserAgreement` contract on Canton and is fully USDCx-enabled.

---

## Step 8 — Mint USDCx (After Ethereum Deposit)

When a user deposits USDC on Ethereum, Circle creates a `DepositAttestation` on Canton. Call `BridgeUserAgreement_Mint` to claim it.

First get the required contract IDs from the factory:

```bash
curl -s -X POST ${UTILITY_BACKEND_URL}/api/utilities/v0/registry/burn-mint-instruction/v0/burn-mint-factory \
  -H "Content-Type: application/json" \
  -d "{
    \"instrumentId\": {
      \"admin\": \"${ADMIN_PARTY_ID}\",
      \"id\": \"USDCx\"
    },
    \"inputHoldingCids\": [],
    \"outputs\": [
      {
        \"owner\": \"${ADMIN_PARTY_ID}\",
        \"amount\": \"<AMOUNT_TO_MINT>\"
      }
    ]
  }"
```

Extract from response:
- `factoryId` → `FACTORY_CID`
- `choiceContext.choiceContextData.values` → `CONTEXT_CONTRACT_IDS`
- `choiceContext.disclosedContracts` → `DISCLOSED_CONTRACTS`

Then submit the mint:

```bash
curl -s -X POST https://<LEDGER_URL>/v2/commands/submit-and-wait-for-transaction \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"commands\": {
      \"commandId\": \"usdcx-mint-$(date +%s)\",
      \"actAs\": [\"$USER_PARTY_ID\"],
      \"readAs\": [\"$USER_PARTY_ID\"],
      \"commands\": [
        {
          \"ExerciseCommand\": {
            \"templateId\": \"#utility-bridge-v0:Utility.Bridge.V0.Agreement.User:BridgeUserAgreement\",
            \"contractId\": \"<BRIDGE_USER_AGREEMENT_CONTRACT_ID>\",
            \"choice\": \"BridgeUserAgreement_Mint\",
            \"choiceArgument\": {
              \"depositAttestationCid\": \"<DEPOSIT_ATTESTATION_CID>\",
              \"factoryCid\": \"<FACTORY_CID>\",
              \"contextContractIds\": <CONTEXT_CONTRACT_IDS>
            }
          }
        }
      ],
      \"disclosedContracts\": <DISCLOSED_CONTRACTS>
    }
  }"
```

> The `FACTORY_CID`, `CONTEXT_CONTRACT_IDS`, and `DISCLOSED_CONTRACTS` values can be cached — they change infrequently. Refresh them periodically rather than on every call.

---

## Step 9 — Burn USDCx (Withdraw to Ethereum)

```bash
curl -s -X POST https://<LEDGER_URL>/v2/commands/submit-and-wait-for-transaction \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"commands\": {
      \"commandId\": \"usdcx-burn-$(date +%s)\",
      \"actAs\": [\"$USER_PARTY_ID\"],
      \"readAs\": [\"$USER_PARTY_ID\"],
      \"commands\": [
        {
          \"ExerciseCommand\": {
            \"templateId\": \"#utility-bridge-v0:Utility.Bridge.V0.Agreement.User:BridgeUserAgreement\",
            \"contractId\": \"<BRIDGE_USER_AGREEMENT_CONTRACT_ID>\",
            \"choice\": \"BridgeUserAgreement_Burn\",
            \"choiceArgument\": {
              \"amount\": \"<AMOUNT_IN_DECIMAL_MAX_6_PLACES>\",
              \"destinationDomain\": \"0\",
              \"destinationRecipient\": \"<ETHEREUM_ADDRESS>\",
              \"holdingCids\": <HOLDING_CONTRACT_IDS>,
              \"requestId\": \"<UUID>\",
              \"reference\": \"\",
              \"factoryCid\": \"<FACTORY_CID>\",
              \"contextContractIds\": <CONTEXT_CONTRACT_IDS>
            }
          }
        }
      ],
      \"disclosedContracts\": <DISCLOSED_CONTRACTS>
    }
  }"
```

> `destinationDomain: "0"` = Ethereum. Only Ethereum is currently supported.
> `amount` supports up to 6 decimal places.
> `requestId` must be a unique UUID per withdrawal request.

---

## Mainnet Checklist

Before running on mainnet, confirm every item:

- [ ] Validator is on `v0.5.12` or later
- [ ] `.env` has mainnet `CROSS_CHAIN_REPRESENTATIVE_PARTY_ID` and `UTILITY_BACKEND_URL`
- [ ] `cub-darsyncer` is in `compose.yaml` and running healthy
- [ ] Both bridge DARs uploaded (`utility-bridge-v0`, `utility-bridge-app-v0`)
- [ ] Correct utility bundle version identified and downloaded for mainnet hash
- [ ] All three registry DARs uploaded (`utility-registry-v0`, `utility-registry-holding-v0`, `utility-registry-app-v0`)
- [ ] nginx `client_max_body_size 50m` is set
- [ ] `BridgeUserAgreementRequest` submitted for each party
- [ ] `BridgeUserAgreement` confirmed via active contracts poll
- [ ] Test mint with smallest possible amount before going live

⚠️ **Mainnet warning:** All transactions involve real USDC. Always test with the smallest possible amount first.

---

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `HTTP 413` on DAR upload | nginx body size limit | Add `client_max_body_size 50m` to `nginx.conf` and reload |
| `failed package name resolution: utility-registry-app-v0` | Registry DAR missing or wrong version | Re-run factory hash check and upload correct bundle version |
| `Missing required field at commands.commandId` | Wrong JSON structure | Use nested `commands` object format as shown in Step 6 |
| `BridgeUserAgreementRequest` stuck pending | Bridge operator approval pending | Normal — poll again after a few minutes |
| DAR upload returns `{}` with `HTTP 200` | DAR already uploaded | Safe to ignore |
| `HTTP 400` on empty payload | Endpoint exists but needs valid body | Expected — not an error |

---

## Key References

- USDCx Wallet Integration: https://docs.digitalasset.com/integrate/devnet/usdcx-support/index.html
- xReserve Workflows: https://docs.digitalasset.com/usdc/xreserve/workflows.html
- Mainnet Technical Setup: https://docs.digitalasset.com/usdc/xreserve/mainnet-technical-setup.html
- TestNet Details: https://docs.digitalasset.com/usdc/xreserve/testnet-technical-setup.html
- Utility DAR Versions: https://docs.digitalasset.com/utilities/devnet/reference/dar-versions/dar-versions.html
- CIP-56 Token Standard: https://github.com/global-synchronizer-foundation/cips/blob/main/cip-0056/cip-0056.md