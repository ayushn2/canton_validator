# Topology Wallet Creation Guide

## What Changes vs Current Code

```bash
CreateWallet()         — unchanged, still works, still hits the 200-wallet cap
CreateExternalWallet() — new scalable flow, bypasses the cap entirely
```

The external wallet flow replaces Steps 4 and 5 only. Steps 1–3 are not needed
for external parties since the topology transaction creates the party directly
(no AllocateParty or CreateUser required).

```bash
Step 1 — Generate Ed25519 key pair            no cap
Step 2 — topology/generate                    no cap  ← creates party via topology
Step 3 — Sign each topology tx hash           local
Step 4 — topology/submit                      no cap  ← registers party on ledger
Step 5 — setup-proposal (create)              no cap  ← operator creates proposal
Step 6 — setup-proposal/prepare-accept        no cap  ← get tx to sign
Step 7 — Sign tx_hash                         local
Step 8 — setup-proposal/submit-accept         no cap  ← activates transfer preapproval
```

No `WalletAppInstall` contract is created. No `ValidatorRight` contract is created.
Neither the 200-wallet Canton cap nor the 100-wallet Dfns cap applies.

---

## Files Added / Changed

| File | Change |
| --- | --- |
| `cantonvalidator/signing.go` | New — Ed25519 key gen + `SignHashHex()` |
| `cantonvalidator/external_party.go` | New — 5 external signing API methods |
| `cantonvalidator/wallet_service.go` | Added `CreateExternalWallet()`, `ExternalWalletUser` type |
| `db/wallet_store.go` | Added `PublicKeyHex`, `PrivateKeyHex` fields to `WalletEntry` |
| `cmd/external_wallet/main.go` | New — entry point |

---

## Key Types

### WalletKeyPair (`cantonvalidator/signing.go`)

```go
type WalletKeyPair struct {
    PublicKeyHex  string // 32 bytes = 64 hex chars
    PrivateKeyHex string // 64 bytes = 128 hex chars
}
```

Keys are **hex-encoded** (not base64). This matches the validator API's expected
format for `public_key` and `signed_hash` fields directly.

### ExternalWalletUser (`cantonvalidator/wallet_service.go`)

```go
type ExternalWalletUser struct {
    PartyID       string
    PublicKeyHex  string
    PrivateKeyHex string
}
```

---

## Signing

```go
func GenerateWalletKeyPair() (*WalletKeyPair, error)
func (kp *WalletKeyPair) SignHashHex(hashHex string) (string, error)
```

`SignHashHex` takes a **hex-encoded hash** (as returned by the API) and returns
a **hex-encoded 64-byte Ed25519 signature** in `${r}${s}` form (128 hex chars).
This is what both `topology/submit` and `setup-proposal/submit-accept` expect.

Uses stdlib `crypto/ed25519` — no external dependencies required.

---

## API Calls

All calls use `cfg.ValidatorURL` and an admin JWT (`GenerateToken(cfg, cfg.LedgerAPIAdminUser)`).

### Step 2 — topology/generate

```bahs
POST /api/validator/v0/admin/external-party/topology/generate

Request:
{
  "party_hint": "my-wallet-1",
  "public_key": "<64-char hex ed25519 public key>"
}

Response:
{
  "party_id": "<party id derived from hint + key fingerprint>",
  "topology_txs": [
    { "topology_tx": "<base64>", "hash": "<hex>" },
    { "topology_tx": "<base64>", "hash": "<hex>" },
    { "topology_tx": "<base64>", "hash": "<hex>" }
  ]
}
```

Returns **three** topology transactions (namespace, party-to-participant mapping,
party-to-key mapping). Each has its own `hash` — sign each independently.

### Step 4 — topology/submit

```bash
POST /api/validator/v0/admin/external-party/topology/submit

Request:
{
  "public_key": "<hex>",
  "signed_topology_txs": [
    { "topology_tx": "<base64 unchanged from generate>", "signed_hash": "<hex r||s>" },
    { "topology_tx": "<base64 unchanged from generate>", "signed_hash": "<hex r||s>" },
    { "topology_tx": "<base64 unchanged from generate>", "signed_hash": "<hex r||s>" }
  ]
}

Response:
{
  "party_id": "<confirmed party id>"
}
```

### Step 5 — setup-proposal (create)

```bash
POST /api/validator/v0/admin/external-party/setup-proposal

Request:  { "user_party_id": "<party_id>" }
Response: { "contract_id": "<ContractId>" }
```

Validator operator creates the `ExternalPartySetupProposal` contract.

### Step 6 — setup-proposal/prepare-accept

```bash
POST /api/validator/v0/admin/external-party/setup-proposal/prepare-accept

Request:
{
  "contract_id":   "<ContractId>",
  "user_party_id": "<party_id>"
}

Response:
{
  "transaction": "<base64 PreparedTransaction protobuf>",
  "tx_hash":     "<hex>"
}
```

Returns the `tx_hash` to sign and the `transaction` to pass through unchanged.

### Step 8 — setup-proposal/submit-accept

```bash
POST /api/validator/v0/admin/external-party/setup-proposal/submit-accept

Request:
{
  "submission": {
    "party_id":       "<party_id>",
    "transaction":    "<base64 unchanged from prepare-accept>",
    "signed_tx_hash": "<hex r||s>",
    "public_key":     "<hex>"
  }
}

Response:
{
  "transfer_preapproval_contract_id": "<ContractId>",
  "update_id": "<string>"
}
```

---

## Usage

```go
client, _ := cantonvalidator.NewCantonGRPCClient()

wallet, err := client.CreateExternalWallet(ctx, "my-wallet-1")
// wallet.PartyID
// wallet.PublicKeyHex
// wallet.PrivateKeyHex  ⚠️ store securely
```

Or run directly:

```bash
go run ./cmd/external_wallet
```

---

## Wallet Store

External wallets are saved to `wallets/wallets.json` with two additional fields:

```json
{
  "name": "my-wallet-1",
  "party_id": "my-wallet-1::...",
  "public_key_hex": "...",
  "private_key_hex": "..."
}
```

Fields `email`, `password`, `auth0_user_id`, `canton_user_id` are omitted
(`omitempty`) since they are not used in the external party flow.

> ⚠️ `private_key_hex` is stored in plaintext for development. Use AWS KMS
> or equivalent for production — replace `SignHashHex` with a KMS signing call.

---

## Verification

After creating an external wallet, confirm it used the topology flow via Canton console:

```bash
docker exec -it splice-validator-participant-1 bash
bin/canton -c /tmp/remote.conf
```

```scala
myparticipant.topology.party_to_participant_mappings.list(
  synchronizerId = myparticipant.synchronizers.list_connected().head.synchronizerId,
  filterParty = "YOUR_FULL_PARTY_ID_HERE"
)
```

✅ Confirms new flow if output shows:

- `signedBy` has two entries (validator key + external party key)
- `SigningKeysWithThreshold` present with `EC-Curve25519` key

---

## What You Gain

| | Standard flow (`CreateWallet`) | External flow (`CreateExternalWallet`) |
| --- | --- | --- |
| Wallet cap | 200 (Canton limit) | None (up to 1M) |
| Validator stays performant at scale | ❌ degrades after ~200 | ✅ unaffected |
| Per-wallet signing key | ❌ | ✅ Ed25519 |
| Blast radius if key compromised | All wallets | 1 wallet |
| You hold the keys | ❌ validator node holds them | ✅ only you can sign |
| KMS / HSM upgrade path | ❌ | ✅ swap `SignHashHex` |
| Wallet UI visible | ✅ | ❌ |
| Transfers | ✅ | ✅ |
| Balance via Ledger API ACS | ✅ | ✅ |
| All gRPC / JSON Ledger API | ✅ | ✅ |
| Transfer preapproval renewal | ✅ | ✅ |
| Auth0 user required | ✅ | ❌ |

### Why the validator stays performant

The standard flow creates two contracts per wallet:

- `WalletAppInstall` — scanned every round by `ValidatorRewardCouponTrigger`
- `ValidatorRight` — scanned every round by `TransferPreapprovalRenewalTrigger` and `WalletSweepTrigger`

At ~200 wallets these triggers take longer than one round (~10 min) to complete,
causing a backlog that eventually crashes the validator app. Topology wallets
create neither contract, so the triggers never see them and the validator
performance is unaffected regardless of how many external wallets exist.

### Why you hold the keys

In the standard flow the validator participant node holds the signing key for
every party it hosts. The node operator can sign any transaction on behalf of
any party on that node.

With external signing the party's signing key never touches the participant.
Only whoever holds the Ed25519 private key can authorize transactions for that
party — the validator operator has no ability to act on your behalf.

### KMS / HSM upgrade path

The signing step is isolated in a single method:

```go
func (kp *WalletKeyPair) SignHashHex(hashHex string) (string, error)
```

To move to AWS KMS or an HSM, replace the body of `SignHashHex` with a KMS
signing call. The rest of `CreateExternalWallet` and all five API methods in
`external_party.go` require no changes — they only consume the hex signature
string that `SignHashHex` returns.
