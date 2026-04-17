# WalletUserProxy — Complete Canton DevNet Runbook

All commands below were tested and verified on Canton DevNet (Splice 0.5.18).

## Prerequisites

- A running Canton validator node (Docker Compose)
- Featured App Rights (self-granted on devnet, or approved by Tokenomics Committee on mainnet)
- `splice-util-featured-app-proxies` DAR uploaded to your participant
- CC balance in your wallet

## Environment Variables

```bash
# SSH into your devnet validator
ssh -L 8080:localhost:80 -i your_key.pem ubuntu@YOUR_EC2_IP
cd ~/splice-node/docker-compose/validator

# Start validator (if not running)
docker compose -f compose.yaml -f compose-expose-ports.yaml -f compose-disable-auth.yaml up -d

# Get auth token (devnet uses unsafe auth)
TOKEN=$(python3 get-token.py administrator)

# Replace these with your actual values
PARTY="your-validator::1220..."
DSO="DSO::1220..."
```

---

## Step 1: Upload Featured App Proxies DAR

```bash
# Copy DAR into participant container
docker compose cp ~/splice-node/dars/splice-util-featured-app-proxies-1.2.2.dar participant:/tmp/

# Upload via Ledger API (inside container, no auth needed)
docker compose exec participant curl -s -o /tmp/resp.json -w "%{http_code}" \
  -X POST "http://localhost:7575/v2/packages" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @/tmp/splice-util-featured-app-proxies-1.2.2.dar

# Expected: 200
```

**Important:** Use `Content-Type: application/octet-stream` with `--data-binary`, NOT JSON base64.

---

## Step 2: Self-Grant Featured App Rights (DevNet Only)

Open the wallet UI at `http://localhost:8080` (via SSH tunnel), log in as `administrator`, and click **"SELF-GRANT FEATURED APP RIGHTS"**.

On mainnet, apply at `canton.foundation/featured-app-request` and wait for Tokenomics Committee approval.

---

## Step 3: Grant User Rights for Ledger API Access

```bash
TOKEN=$(python3 get-token.py administrator)

# Grant CanReadAs for your validator party
curl -s -X POST "http://localhost:7575/v2/users/administrator/rights" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "rights": [{
      "kind": {
        "CanReadAs": {
          "value": {
            "party": "YOUR_PARTY_ID"
          }
        }
      }
    }],
    "userId": "administrator"
  }'

# Expected: {"newlyGrantedRights":[{"kind":{"CanReadAs":...}}]}
```

---

## Step 4: Create WalletUserProxy Contract

```bash
TOKEN=$(python3 get-token.py administrator)

curl -s -X POST "http://localhost:7575/v2/commands/submit-and-wait" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "commands": [{
      "CreateCommand": {
        "templateId": "2889c094cf9678b2b666221934ea56ab169a31b257450845bd53217a8cdfe44f:Splice.Util.FeaturedApp.WalletUserProxy:WalletUserProxy",
        "createArguments": {
          "provider": "YOUR_PARTY_ID",
          "providerWeight": "1.0",
          "userWeight": "0.0",
          "extraBeneficiaries": [],
          "optAllowList": null
        }
      }
    }],
    "actAs": ["YOUR_PARTY_ID"],
    "userId": "administrator",
    "commandId": "create-wallet-user-proxy-1"
  }'

# Expected: {"updateId":"1220...","completionOffset":NNNNN}
```

**Important notes:**

- `providerWeight` and `userWeight` must be Daml Decimal format: `"1.0"` not `"1"` or `"1000000"`
- `providerWeight: 1.0, userWeight: 0.0` = 100% rewards to provider
- Template fields are ONLY: `provider`, `providerWeight`, `userWeight`, `extraBeneficiaries`, `optAllowList`
- There is NO `dso` or `user` field in createArguments

---

## Step 5: Find Created Contract IDs

### Find WalletUserProxy Contract ID

Use the `completionOffset` from Step 4 (replace NNNNN with actual value):

```bash
TOKEN=$(python3 get-token.py administrator)

curl -s "http://localhost:7575/v2/updates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "filtersByParty": {
        "YOUR_PARTY_ID": {
          "templateFilters": [{
            "templateId": "2889c094cf9678b2b666221934ea56ab169a31b257450845bd53217a8cdfe44f:Splice.Util.FeaturedApp.WalletUserProxy:WalletUserProxy",
            "includeCreatedEventBlob": true
          }]
        }
      }
    },
    "beginExclusive": "OFFSET_MINUS_1",
    "endInclusive": "OFFSET_PLUS_5"
  }' | python3 -c "
import sys, json
for item in json.loads(sys.stdin.read()):
    txn = item.get('update',{}).get('Transaction',{}).get('value',{})
    for e in txn.get('events',[]):
        ce = e.get('CreatedEvent',{})
        if 'WalletUserProxy' in ce.get('templateId',''):
            print(f'WalletUserProxy CID: {ce[\"contractId\"]}')"
```

### Find FeaturedAppRight Contract ID

Search the transaction history around the time you self-granted:

```bash
TOKEN=$(python3 get-token.py administrator)

curl -s "http://localhost:7575/v2/updates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "filtersByParty": {
        "YOUR_PARTY_ID": {
          "cumulative": [{"identifierFilter": {"WildcardFilter": {"value": {}}}}]
        }
      }
    },
    "beginExclusive": "OFFSET_BEFORE_SELF_GRANT",
    "endInclusive": "OFFSET_AFTER_SELF_GRANT"
  }' | python3 -c "
import sys, json
for item in json.loads(sys.stdin.read()):
    txn = item.get('update',{}).get('Transaction',{}).get('value',{})
    for e in txn.get('events',[]):
        ce = e.get('CreatedEvent',{})
        if 'FeaturedAppRight' in ce.get('templateId',''):
            print(f'FeaturedAppRight CID: {ce[\"contractId\"]}')"
```

Save both contract IDs — you'll need them for every transfer.

---

## Step 6: Execute Proxy Transfer (Complete Working Script)

```python
#!/usr/bin/env python3
"""
proxy_transfer.py
Transfer CC through WalletUserProxy to create FeaturedAppActivityMarker and earn rewards.

Usage: python3 proxy_transfer.py
"""
import json, sys, urllib.request, subprocess, uuid
from datetime import datetime, timezone, timedelta

# ── Configuration ────────────────────────────────────────────────
LEDGER_API = "http://localhost:7575"
SCAN_PROXY = "http://localhost:80/api/validator/v0/scan-proxy"
WALLET     = "http://localhost:80/api/validator/v0/wallet"

PARTY     = "YOUR_VALIDATOR_PARTY_ID"            # your validator party
DSO       = "DSO::1220..."                        # DSO party (get from scan-proxy/amulet-rules)
PROXY_CID = "YOUR_WALLET_USER_PROXY_CONTRACT_ID"  # from Step 5
FAR_CID   = "YOUR_FEATURED_APP_RIGHT_CONTRACT_ID" # from Step 5
RECEIVER  = PARTY                                  # self-transfer for testing

# ── Helpers ──────────────────────────────────────────────────────
token = subprocess.check_output(["python3", "get-token.py", "administrator"]).decode().strip()

def api_call(url, data=None, method=None):
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    if data:
        req = urllib.request.Request(url, json.dumps(data).encode(), headers, method=method or "POST")
    else:
        req = urllib.request.Request(url, headers=headers, method=method or "GET")
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.request.HTTPError as e:
        print(f"ERROR {e.code}: {e.read().decode()[:600]}")
        return None

# ── Step 1: Get CC Holding (UTXO) ───────────────────────────────
print("Step 1: Getting CC holding...")
amulets = api_call(f"{WALLET}/amulets")
holding_cid  = amulets["amulets"][0]["contract"]["contract"]["contract_id"]
holding_blob = amulets["amulets"][0]["contract"]["contract"].get("created_event_blob", "")
print(f"  Holding: {holding_cid[:40]}...")

# ── Step 2: Query Registry for TransferFactory ───────────────────
print("Step 2: Querying registry for TransferFactory...")
now     = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.000000Z")
expires = (datetime.now(timezone.utc) + timedelta(hours=24)).strftime("%Y-%m-%dT%H:%M:%S.000000Z")

registry_resp = api_call(
    f"{SCAN_PROXY}/registry/transfer-instruction/v1/transfer-factory",
    {
        "instrumentId": {"admin": DSO, "id": "Amulet"},
        "choiceArguments": {
            "expectedAdmin": DSO,
            "transfer": {
                "sender": PARTY,
                "receiver": RECEIVER,
                "amount": "1.0",
                "instrumentId": {"admin": DSO, "id": "Amulet"},
                "requestedAt": now,
                "executeBefore": expires,
                "meta": {"values": {}},
                "inputHoldingCids": [holding_cid]
            },
            "extraArgs": {
                "context": {"values": {}},
                "meta": {"values": {}}
            }
        }
    }
)
if not registry_resp:
    print("Registry call failed!")
    sys.exit(1)

factory_cid    = registry_resp["factoryId"]
choice_context = registry_resp["choiceContext"]["choiceContextData"]
disclosed_from_registry = registry_resp["choiceContext"]["disclosedContracts"]
print(f"  FactoryID: {factory_cid[:40]}...")
print(f"  Disclosed contracts from registry: {len(disclosed_from_registry)}")

# ── Step 3: Build Disclosed Contracts ────────────────────────────
disclosed = []
for dc in disclosed_from_registry:
    if dc.get("createdEventBlob"):
        disclosed.append({"createdEventBlob": dc["createdEventBlob"]})
if holding_blob:
    disclosed.append({"createdEventBlob": holding_blob})
print(f"  Total disclosed contracts: {len(disclosed)}")

# ── Step 4: Submit WalletUserProxy_TransferFactory_Transfer ──────
print("Step 3: Submitting proxy transfer...")

choice_arg = {
    "cid": factory_cid,
    "proxyArg": {
        "user": PARTY,
        "featuredAppRightCid": FAR_CID,
        "choiceArg": {
            "expectedAdmin": DSO,
            "transfer": {
                "sender": PARTY,
                "receiver": RECEIVER,
                "amount": "1.0",
                "instrumentId": {"admin": DSO, "id": "Amulet"},
                "requestedAt": now,
                "executeBefore": expires,
                "meta": {"values": {}},
                "inputHoldingCids": [holding_cid]
            },
            "extraArgs": {
                "context": choice_context,
                "meta": {"values": {}}
            }
        }
    }
}

command = {
    "commands": [{
        "ExerciseCommand": {
            "templateId": "2889c094cf9678b2b666221934ea56ab169a31b257450845bd53217a8cdfe44f:Splice.Util.FeaturedApp.WalletUserProxy:WalletUserProxy",
            "contractId": PROXY_CID,
            "choice": "WalletUserProxy_TransferFactory_Transfer",
            "choiceArgument": choice_arg
        }
    }],
    "actAs": [PARTY],
    "readAs": [PARTY],
    "userId": "administrator",
    "commandId": f"proxy-transfer-{uuid.uuid4().hex[:8]}",
    "disclosedContracts": disclosed
}

result = api_call(f"{LEDGER_API}/v2/commands/submit-and-wait", command)
if result:
    print(f"\nSUCCESS!")
    print(f"  UpdateID: {result.get('updateId','?')}")
    print(f"  Offset:   {result.get('completionOffset','?')}")
else:
    print("\nFAILED")
```

---

## Step 7: Verify FeaturedAppActivityMarker Was Created

Replace `OFFSET` with one less than the `completionOffset` from Step 6:

```bash
TOKEN=$(python3 get-token.py administrator)

curl -s "http://localhost:7575/v2/updates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "filtersByParty": {
        "YOUR_PARTY_ID": {
          "cumulative": [{"identifierFilter": {"WildcardFilter": {"value": {}}}}]
        }
      }
    },
    "beginExclusive": "OFFSET",
    "endInclusive": "OFFSET_PLUS_5"
  }' | python3 -c "
import sys, json
for item in json.loads(sys.stdin.read()):
    txn = item.get('update',{}).get('Transaction',{}).get('value',{})
    if not isinstance(txn, dict): continue
    for e in txn.get('events',[]):
        ce = e.get('CreatedEvent',{})
        if ce and isinstance(ce, dict):
            tid = ce.get('templateId','').split(':')[-1]
            print(f'CREATED: {tid}')
            if 'Marker' in tid:
                print(f'  *** FEATURED APP ACTIVITY MARKER ***')
                print(f'  {json.dumps(ce.get(\"createArgument\",{}))[:300]}')
"
```

Expected output:

```bash
CREATED: FeaturedAppActivityMarker
  *** FEATURED APP ACTIVITY MARKER ***
  {"dso":"DSO::...","provider":"your-validator::...","beneficiary":"your-validator::...","weight":"1.0000000000"}
CREATED: Amulet
CREATED: Amulet
```

---

## Step 8: Verify Marker → AppRewardCoupon Conversion

Wait ~10 minutes for SV automation, then:

```bash
TOKEN=$(python3 get-token.py administrator)
END=$(curl -s "http://localhost:7575/v2/state/ledger-end" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.loads(sys.stdin.read())['offset'])")

curl -s "http://localhost:7575/v2/updates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "filtersByParty": {
        "YOUR_PARTY_ID": {
          "cumulative": [{"identifierFilter": {"WildcardFilter": {"value": {}}}}]
        }
      }
    },
    "beginExclusive": "TRANSFER_OFFSET",
    "endInclusive": "'"$END"'"
  }' | python3 -c "
import sys, json
for item in json.loads(sys.stdin.read()):
    txn = item.get('update',{}).get('Transaction',{}).get('value',{})
    if not isinstance(txn, dict): continue
    for e in txn.get('events',[]):
        for key in ['CreatedEvent','ArchivedEvent']:
            ev = e.get(key,{})
            if ev and isinstance(ev, dict):
                tid = ev.get('templateId','').split(':')[-1]
                if 'Marker' in tid or 'Coupon' in tid or 'AppReward' in tid:
                    action = 'CREATED' if key == 'CreatedEvent' else 'ARCHIVED'
                    print(f'{action} {tid} at offset {txn.get(\"offset\",\"?\")}')
"
```

Expected output:

```bash
ARCHIVED FeaturedAppActivityMarker at offset NNNNN
CREATED AppRewardCoupon at offset NNNNN
```

---

## Utility Commands

### Check CC Balance

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s "http://localhost:80/api/validator/v0/wallet/balance" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

### Get Current Ledger End

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s "http://localhost:7575/v2/state/ledger-end" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

### Get AmuletRules (includes DSO party and config)

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s "http://localhost:80/api/validator/v0/scan-proxy/amulet-rules" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; d=json.loads(sys.stdin.read())
print(f'DSO: {d[\"amulet_rules\"][\"contract\"][\"payload\"][\"dso\"]}')
print(f'CID: {d[\"amulet_rules\"][\"contract\"][\"contract_id\"][:60]}...')
print(f'extraFeaturedAppRewardAmount: {d[\"amulet_rules\"][\"contract\"][\"payload\"][\"configSchedule\"][\"initialValue\"][\"transferConfig\"][\"extraFeaturedAppRewardAmount\"]}')"
```

### Get Open Mining Round

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s "http://localhost:80/api/validator/v0/scan-proxy/open-and-issuing-mining-rounds" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; d=json.loads(sys.stdin.read())
r=d['open_mining_rounds'][0]
print(f'Round: {r[\"contract\"][\"payload\"][\"round\"][\"number\"]}')
print(f'CID: {r[\"contract\"][\"contract_id\"][:60]}...')"
```

### Get CC Holdings (Amulet UTXOs)

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s "http://localhost:80/api/validator/v0/wallet/amulets" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; d=json.loads(sys.stdin.read())
for a in d['amulets']:
    c=a['contract']['contract']
    print(f'CID: {c[\"contract_id\"][:60]}...')
    print(f'Amount: {c[\"payload\"][\"amount\"][\"initialAmount\"]}')"
```

### Get TransferFactory from Registry

```bash
TOKEN=$(python3 get-token.py administrator)
curl -s -X POST \
  "http://localhost:80/api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "instrumentId": {"admin": "YOUR_DSO_PARTY", "id": "Amulet"},
    "choiceArguments": {
      "expectedAdmin": "YOUR_DSO_PARTY",
      "transfer": {
        "sender": "SENDER_PARTY",
        "receiver": "RECEIVER_PARTY",
        "amount": "1.0",
        "instrumentId": {"admin": "YOUR_DSO_PARTY", "id": "Amulet"},
        "requestedAt": "2026-04-17T15:00:00.000000Z",
        "executeBefore": "2026-04-18T15:00:00.000000Z",
        "meta": {"values": {}},
        "inputHoldingCids": ["HOLDING_CID"]
      },
      "extraArgs": {"context": {"values": {}}, "meta": {"values": {}}}
    }
  }' | python3 -c "
import sys,json; d=json.loads(sys.stdin.read())
print(f'factoryId: {d[\"factoryId\"][:60]}...')
print(f'disclosed contracts: {len(d[\"choiceContext\"][\"disclosedContracts\"])}')"
```

---

## WalletUserProxy_TransferFactory_Transfer — JSON Structure Reference

This is the exact JSON structure for the `choiceArgument` field:

```json
{
  "cid": "<factoryId from registry — NOT AmuletRules>",
  "proxyArg": {
    "user": "<sender party ID>",
    "featuredAppRightCid": "<FeaturedAppRight contract ID>",
    "choiceArg": {
      "expectedAdmin": "<DSO party ID>",
      "transfer": {
        "sender": "<sender party ID>",
        "receiver": "<receiver party ID>",
        "amount": "<CC amount as string>",
        "instrumentId": {
          "admin": "<DSO party ID>",
          "id": "Amulet"
        },
        "requestedAt": "<ISO 8601 timestamp>",
        "executeBefore": "<ISO 8601 timestamp>",
        "meta": {"values": {}},
        "inputHoldingCids": ["<Amulet UTXO contract IDs>"]
      },
      "extraArgs": {
        "context": "<choiceContextData from registry response>",
        "meta": {"values": {}}
      }
    }
  }
}
```

Additionally, the `submit-and-wait` command must include:

- `disclosedContracts`: all `createdEventBlob` values from the registry response + the holding blob
- `actAs` and `readAs`: both set to the sender's party
- `templateId`: the WalletUserProxy template ID (not an interface)

---

## Key Discoveries & Gotchas

| What | Correct Value | Common Mistake |
| ------ | -------------- | ---------------- |
| Instrument ID for CC | `"Amulet"` | `"CantonCoin"` (display name only) |
| `cid` in choiceArgument | `factoryId` from registry | AmuletRules CID (wrong — doesn't implement TransferFactory interface) |
| Registry endpoint | `POST .../registry/transfer-instruction/v1/transfer-factory` | `GET` (returns 405), wrong path |
| Weight format | `"1.0"` (Daml Decimal string) | `1` (integer), `"1000000"` |
| Empty TextMap in JSON | `{"values": {}}` | `{}`, `{"values": []}`, `{"map": {}}` |
| `extraArgs.context` | Use `choiceContextData` from registry response | Empty `{"values": {}}` (works for registry call, but use returned context for submit) |
| DAR upload Content-Type | `application/octet-stream` | `application/json` with base64 |
| WalletUserProxy template fields | `provider`, `providerWeight`, `userWeight`, `extraBeneficiaries`, `optAllowList` | Adding `dso` or `user` field |
| ProxyArg field names in choiceArgument | `choiceArg` (inside proxyArg) | `arg` or `factoryCid` |
| Timestamps | ISO 8601: `"2026-04-17T15:00:00.000000Z"` | Microseconds since epoch |
| Disclosed contracts | Required — from registry + holding blob | Omitting them → CONTRACT_NOT_FOUND |

---

## Proven Reward Flow

```bash
1. User transfer via WalletUserProxy_TransferFactory_Transfer
     → CREATED FeaturedAppActivityMarker
       {provider: "your-validator", beneficiary: "your-validator", weight: "1.0"}

2. ~10 minutes later, SV automation picks up marker
     → ARCHIVED FeaturedAppActivityMarker
     → CREATED AppRewardCoupon

3. Validator automation mints CC from coupon
     → CC appears in your balance
```
