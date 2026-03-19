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

### Four Operations

| Operation | Trigger | Frequency |
| --- | --- | --- |
| **Onboard** | Register party with xReserve bridge | Once per party |
| **Mint** | User deposits USDC on Ethereum | On demand |
| **Check Balance** | Query USDCx holdings on Canton | On demand |
| **Burn** | User withdraws USDCx to Ethereum | On demand |

---

## Environment Variables

The only values that differ between testnet and mainnet are the party IDs and backend URL. Everything else — commands, DAR names, API paths — is identical.

### Testnet

```bash
UTILITY_BACKEND_URL=https://api.utilities.digitalasset-staging.com
ADMIN_PARTY_ID=decentralized-usdc-interchain-rep::122049e2af8a725bd19759320fc83c638e7718973eac189d8f201309c512d1ffec61
UTILITY_OPERATOR_PARTY_ID=DigitalAsset-UtilityOperator::12202679f2bbe57d8cba9ef3cee847ac8239df0877105ab1f01a77d47477fdce1204
BRIDGE_OPERATOR_PARTY_ID=Bridge-Operator::12209d011ce250de439fefc35d16d1ab9d56fb99ccb24c18d798efb22352d533bcdb
```

### Mainnet

```bash
UTILITY_BACKEND_URL=https://api.utilities.digitalasset.com
ADMIN_PARTY_ID=decentralized-usdc-interchain-rep::12208115f1e168dd7e792320be9c4ca720c751a02a3053c7606e1c1cd3dad9bf60ef
UTILITY_OPERATOR_PARTY_ID=auth0_007c6643538f2eadd3e573dd05b9::12205bcc106efa0eaa7f18dc491e5c6f5fb9b0cc68dc110ae66f4ed6467475d7c78e
BRIDGE_OPERATOR_PARTY_ID=Bridge-Operator::1220c8448890a70e65f6906bd48d797ee6551f094e9e6a53e329fd5b2b549334f13f
```

### Your validator (fill in)

```bash
VALIDATOR_URL=https://<your-validator-domain>
LEDGER_URL=https://<your-ledger-api-domain>
TOKEN=<your-jwt-token>
USER_PARTY_ID=<party-id-to-onboard>
SYNC_ID=<your-synchronizer-id>
```

> `SYNC_ID` looks like `global-domain::1220f22a8b8f...`. Get it by running:
>
> ```bash
> curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
>   -H "Authorization: Bearer $TOKEN" \
>   -H "Content-Type: application/json" \
>   -d '{"filter":{"filtersForAnyParty":{"cumulative":[{"identifierFilter":{"WildcardFilter":{"value":{"includeCreatedEventBlob":false}}}}]}},"verbose":false,"activeAtOffset":1}' \
>   | python3 -c "import sys,re; print(re.search(r'synchronizerId[\":]+([^\"]+)\"', sys.stdin.read()).group(1))"
> ```

Throughout this guide, replace all `<placeholders>` with your values. All party IDs in the environment variable tables above are fixed — do not change them.

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

# Verify
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

> Replace `CLIENT_ID`, `CLIENT_SECRET`, and `OAUTH_DOMAIN` with your M2M credentials. These are the same credentials your validator uses to authenticate with the ledger API.

Validate, pull, and start:

```bash
# Validate compose file
docker compose config --quiet && echo "✅ valid" || echo "❌ errors"

# Pull the image
docker compose pull cub-darsyncer

# Start darsyncer only
docker compose up -d cub-darsyncer

# Watch logs — confirm both DARs uploaded successfully
docker logs -f <cub-darsyncer-container-name> 2>&1 | head -50
```

Expected output:

```bash
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
<https://docs.digitalasset.com/utilities/devnet/reference/dar-versions/dar-versions.html>

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
# Get current ledger end offset (required for this API version)
OFFSET=$(curl -s https://<LEDGER_URL>/v2/state/ledger-end \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin)['offset'])")

echo "Current offset: $OFFSET"

# Poll for BridgeUserAgreement contract
curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [
            {
              \"identifierFilter\": {
                \"WildcardFilter\": {
                  \"value\": {
                    \"includeCreatedEventBlob\": false
                  }
                }
              }
            }
          ]
        }
      }
    },
    \"verbose\": false,
    \"activeAtOffset\": $OFFSET
  }" | python3 -c "
import sys
raw = sys.stdin.read()
if 'BridgeUserAgreement' in raw and 'Request' not in raw[raw.find('BridgeUserAgreement'):raw.find('BridgeUserAgreement')+30]:
    print('✅ Approved — party is fully onboarded to xReserve')
elif 'BridgeUserAgreementRequest' in raw:
    print('⏳ Request exists but not yet approved — try again in a few minutes')
else:
    print('⏳ Pending approval — try again in a few minutes')
"
```

> **Note:** The `activeAtOffset` parameter is required for this API version. Always fetch the current ledger end offset first before querying active contracts.

Once approved, the party has a `BridgeUserAgreement` contract on Canton and is fully USDCx-enabled.

---

## Step 8 — Deposit USDC on Ethereum (to get USDCx)

Before minting, you need real USDC on Ethereum (or testnet USDC on Sepolia for testnet).

### Testnet — Get testnet USDC and ETH

1. Get Sepolia ETH (for gas):
   - <https://cloud.google.com/application/web3/faucet/ethereum/sepolia>
   - <https://sepoliafaucet.com>

2. Get testnet USDC from Circle:
   - <https://faucet.circle.com> — select Sepolia, paste your MetaMask address

3. Add testnet USDC to MetaMask (import token):

   ```bash
   Contract: 0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238
   ```

### Deposit via xReserve UI

1. Go to: <https://digital-asset.github.io/xreserve-deposits/>
2. Connect your MetaMask wallet
3. Enter the **Canton recipient party ID** (`USER_PARTY_ID`)
4. Enter the amount (start small — e.g. `1`)
5. Click **Deposit** — approve 2 MetaMask transactions:
   - Transaction 1: Approve USDC spending allowance
   - Transaction 2: depositToRemote to xReserve contract
6. Wait **13-15 minutes** for Ethereum finality

### Check for DepositAttestation on Canton

After waiting, poll until Circle creates the `DepositAttestation`:

```bash
OFFSET=$(curl -s https://<LEDGER_URL>/v2/state/ledger-end \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin)['offset'])")

curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [{
            \"identifierFilter\": {
              \"WildcardFilter\": {
                \"value\": {\"includeCreatedEventBlob\": false}
              }
            }
          }]
        }
      }
    },
    \"verbose\": false,
    \"activeAtOffset\": $OFFSET
  }" | python3 -c "
import sys, re
raw = sys.stdin.read()
if 'DepositAttestation' in raw:
    print('✅ DepositAttestation found - ready to mint!')
    idx = raw.find('DepositAttestation')
    match = re.search(r'\"contractId\":\"([^\"]+)\"', raw[max(0,idx-300):idx+100])
    if match:
        print('DepositAttestation CID:', match.group(1))
else:
    print('⏳ Not yet — wait a few more minutes and try again')
"
```

---

## Step 9 — Mint USDCx

Once the `DepositAttestation` appears, call `BridgeUserAgreement_Mint` to claim the USDC as USDCx holdings on Canton.

### Get required contract IDs

```bash
# Get DepositAttestation CID
DEPOSIT_ATTESTATION_CID=$(curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [{\"identifierFilter\": {\"WildcardFilter\": {\"value\": {\"includeCreatedEventBlob\": false}}}}]
        }
      }
    },
    \"verbose\": false,
    \"activeAtOffset\": $OFFSET
  }" | python3 -c "
import sys, re
raw = sys.stdin.read()
idx = raw.find('DepositAttestation')
match = re.search(r'\"contractId\":\"([^\"]+)\"', raw[max(0,idx-300):idx+100])
if match: print(match.group(1))
")

# Get BridgeUserAgreement CID
BRIDGE_AGREEMENT_CID=$(curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [{\"identifierFilter\": {\"WildcardFilter\": {\"value\": {\"includeCreatedEventBlob\": false}}}}]
        }
      }
    },
    \"verbose\": false,
    \"activeAtOffset\": $OFFSET
  }" | python3 -c "
import sys, re
raw = sys.stdin.read()
idx = raw.find('BridgeUserAgreement\"')
match = re.search(r'\"contractId\":\"([^\"]+)\"', raw[max(0,idx-300):idx+100])
if match: print(match.group(1))
")

# Get factory context
FACTORY_RESPONSE=$(curl -s -X POST ${UTILITY_BACKEND_URL}/api/utilities/v0/registry/burn-mint-instruction/v0/burn-mint-factory \
  -H "Content-Type: application/json" \
  -d "{
    \"instrumentId\": {\"admin\": \"${ADMIN_PARTY_ID}\", \"id\": \"USDCx\"},
    \"inputHoldingCids\": [],
    \"outputs\": []
  }")

FACTORY_CID=$(echo $FACTORY_RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['factoryId'])")

CONTEXT_IDS=$(echo $FACTORY_RESPONSE | python3 -c "
import sys, json
data = json.load(sys.stdin)
values = data['choiceContext']['choiceContextData']['values']
print(json.dumps({
    'instrumentConfigurationCid': values['utility.digitalasset.com/instrument-configuration']['value'],
    'appRewardConfigurationCid': values['utility.digitalasset.com/app-reward-configuration']['value'],
    'featuredAppRightCid': values['utility.digitalasset.com/featured-app-right']['value']
}))
")

# Add synchronizerId to disclosed contracts (required by this API version)
DISCLOSED=$(echo $FACTORY_RESPONSE | python3 -c "
import sys, json
data = json.load(sys.stdin)
contracts = data['choiceContext']['disclosedContracts']
for c in contracts:
    c['synchronizerId'] = '<SYNC_ID>'
print(json.dumps(contracts))
")

echo "DepositAttestation CID: $DEPOSIT_ATTESTATION_CID"
echo "BridgeAgreement CID:    $BRIDGE_AGREEMENT_CID"
echo "Factory CID:            $FACTORY_CID"
```

> **Important:** Replace `<SYNC_ID>` in the `DISCLOSED` extraction above with your actual synchronizer ID.

### Submit the Mint

```bash
curl -s -w "\nHTTP:%{http_code}" \
  -X POST https://<LEDGER_URL>/v2/commands/submit-and-wait-for-transaction \
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
            \"contractId\": \"$BRIDGE_AGREEMENT_CID\",
            \"choice\": \"BridgeUserAgreement_Mint\",
            \"choiceArgument\": {
              \"depositAttestationCid\": \"$DEPOSIT_ATTESTATION_CID\",
              \"factoryCid\": \"$FACTORY_CID\",
              \"contextContractIds\": $CONTEXT_IDS
            }
          }
        }
      ],
      \"disclosedContracts\": $DISCLOSED
    }
  }"
```

Expected: `HTTP:200` with a `CreatedEvent` for `Utility.Registry.Holding.V0.Holding:Holding` — this is your USDCx holding.

---

## Step 10 — Check USDCx Balance

Query your active USDCx holdings on Canton:

```bash
OFFSET=$(curl -s https://<LEDGER_URL>/v2/state/ledger-end \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin)['offset'])")

curl -s -X POST https://<LEDGER_URL>/v2/state/active-contracts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filter\": {
      \"filtersByParty\": {
        \"$USER_PARTY_ID\": {
          \"cumulative\": [{
            \"identifierFilter\": {
              \"WildcardFilter\": {
                \"value\": {\"includeCreatedEventBlob\": false}
              }
            }
          }]
        }
      }
    },
    \"verbose\": false,
    \"activeAtOffset\": $OFFSET
  }" | python3 -c "
import sys, json
raw = sys.stdin.read()
data = json.loads(raw)
total = 0
holding_cids = []
for entry in data:
    contract = entry.get('contractEntry', {}).get('JsActiveContract', {})
    created = contract.get('createdEvent', {})
    template = created.get('templateId', '')
    args = created.get('createArgument', {})
    if 'USDCx' in str(args) or 'Holding' in template:
        amount = float(args.get('amount', 0))
        total += amount
        holding_cids.append(created.get('contractId', ''))
        print(f'Holding: {amount} USDCx — CID: {created.get(\"contractId\", \"\")[:40]}...')
print(f'Total USDCx balance: {total}')
print(f'Holding contract IDs: {holding_cids}')
"
```

> **Note:** USDCx holdings do NOT appear in the standard Canton wallet UI — the wallet UI only shows Canton Coin (CC). USDCx must be queried via the ledger API as shown above.

---

## Step 11 — Burn USDCx (Withdraw to Ethereum)

To withdraw USDCx back to Ethereum, burn the holding on Canton. Get your holding contract IDs from Step 10 first.

```bash
# Get fresh factory context for burn
FACTORY_RESPONSE=$(curl -s -X POST ${UTILITY_BACKEND_URL}/api/utilities/v0/registry/burn-mint-instruction/v0/burn-mint-factory \
  -H "Content-Type: application/json" \
  -d "{
    \"instrumentId\": {\"admin\": \"${ADMIN_PARTY_ID}\", \"id\": \"USDCx\"},
    \"inputHoldingCids\": [\"<HOLDING_CONTRACT_ID>\"],
    \"outputs\": []
  }")

FACTORY_CID=$(echo $FACTORY_RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['factoryId'])")
CONTEXT_IDS=$(echo $FACTORY_RESPONSE | python3 -c "
import sys, json
data = json.load(sys.stdin)
values = data['choiceContext']['choiceContextData']['values']
print(json.dumps({
    'instrumentConfigurationCid': values['utility.digitalasset.com/instrument-configuration']['value'],
    'appRewardConfigurationCid': values['utility.digitalasset.com/app-reward-configuration']['value'],
    'featuredAppRightCid': values['utility.digitalasset.com/featured-app-right']['value']
}))
")
DISCLOSED=$(echo $FACTORY_RESPONSE | python3 -c "
import sys, json
data = json.load(sys.stdin)
contracts = data['choiceContext']['disclosedContracts']
for c in contracts:
    c['synchronizerId'] = '<SYNC_ID>'
print(json.dumps(contracts))
")

# Submit Burn
curl -s -w "\nHTTP:%{http_code}" \
  -X POST https://<LEDGER_URL>/v2/commands/submit-and-wait-for-transaction \
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
            \"contractId\": \"$BRIDGE_AGREEMENT_CID\",
            \"choice\": \"BridgeUserAgreement_Burn\",
            \"choiceArgument\": {
              \"amount\": \"<AMOUNT_MAX_6_DECIMAL_PLACES>\",
              \"destinationDomain\": \"0\",
              \"destinationRecipient\": \"<ETHEREUM_ADDRESS>\",
              \"holdingCids\": [\"<HOLDING_CONTRACT_ID>\"],
              \"requestId\": \"$(python3 -c 'import uuid; print(uuid.uuid4())')\",
              \"reference\": \"\",
              \"factoryCid\": \"$FACTORY_CID\",
              \"contextContractIds\": $CONTEXT_IDS
            }
          }
        }
      ],
      \"disclosedContracts\": $DISCLOSED
    }
  }"
```

> `destinationDomain: "0"` = Ethereum. Only Ethereum is currently supported.
> `amount` supports up to 6 decimal places.
> `requestId` must be a unique UUID per withdrawal — never reuse.
> After burning, USDC will appear in your Ethereum wallet within minutes of Canton finality.

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
- [ ] `SYNC_ID` confirmed for your mainnet synchronizer
- [ ] Test mint with smallest possible amount before going live

⚠️ **Mainnet warning:** All transactions involve real USDC. Always test with the smallest possible amount first.

---

## Troubleshooting

| Error | Cause | Fix |
| --- | --- | --- |
| `HTTP 413` on DAR upload | nginx body size limit | Add `client_max_body_size 50m` to `nginx.conf` and reload |
| `failed package name resolution: utility-registry-app-v0` | Registry DAR missing or wrong version | Re-run factory hash check and upload correct bundle version |
| `Missing required field at commands.commandId` | Wrong JSON structure | Use nested `commands` object format as shown in Step 6 |
| `Missing required field at synchronizerId` | Disclosed contracts missing synchronizer ID | Add `synchronizerId` field to each disclosed contract |
| `Missing required field at value` | Wrong filter structure for active-contracts | Use `WildcardFilter: { value: { includeCreatedEventBlob: false } }` format |
| `BridgeUserAgreementRequest` stuck pending | Bridge operator approval pending | Normal — poll again after a few minutes |
| DAR upload returns `{}` with `HTTP 200` | DAR already uploaded | Safe to ignore |
| `HTTP 400` on empty payload | Endpoint exists but needs valid body | Expected — not an error |
| USDCx not visible in wallet UI | Wallet UI only shows CC | Query holdings via ledger API as shown in Step 10 |
| `DepositAttestation` not appearing | Ethereum finality not reached yet | Wait full 15 minutes after Ethereum tx confirmed |

---

## Key References

- USDCx Wallet Integration: <https://docs.digitalasset.com/integrate/devnet/usdcx-support/index.html>
- xReserve Workflows: <https://docs.digitalasset.com/usdc/xreserve/workflows.html>
- Mainnet Technical Setup: <https://docs.digitalasset.com/usdc/xreserve/mainnet-technical-setup.html>
- TestNet Details: <https://docs.digitalasset.com/usdc/xreserve/testnet-technical-setup.html>
- Utility DAR Versions: <https://docs.digitalasset.com/utilities/devnet/reference/dar-versions/dar-versions.html>
- CIP-56 Token Standard: <https://github.com/global-synchronizer-foundation/cips/blob/main/cip-0056/cip-0056.md>
- xReserve Deposit UI (testnet): <https://digital-asset.github.io/xreserve-deposits/>
- Circle Testnet Faucet: <https://faucet.circle.com>
