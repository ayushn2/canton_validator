# Canton Wallet Scaling Guide

---

## The Core Distinction

| Term | What It Is | Cap |
| --- | --- | --- |
| **Party** | A Canton ledger identity (`AllocateParty`) | None — up to 1 million per node |
| **Wallet** | A party + signing key + preapproval setup | Capped (see below) |

A party becomes a wallet only after Steps 4 and 5 of the creation flow. Steps 1–3 are unlimited regardless of approach.

---

## The 200-Party Limit (Canton Validator App)

The Canton **participant** supports up to 1 million parties per node. However, the **validator app** has a separate limit of 200 parties due to background automation triggers that run every round (~10 min):

- `ValidatorRewardCouponTrigger` — scans all `WalletAppInstall` contracts
- `TransferPreapprovalRenewalTrigger` — scans all `ValidatorRight` contracts
- `WalletSweepTrigger` — scans all wallet balances

At 200 parties, these triggers take longer than one round to complete, causing backlogs and eventually crashing the validator app.

**What triggers the 200 limit:**

- `/api/validator/v0/register` → creates `WalletAppInstall` ← counted
- `/api/validator/v0/wallet/transfer-preapproval` → creates `ValidatorRight` ← counted

**What does NOT trigger the 200 limit:**

- `AllocateParty` (gRPC) — just a ledger identity, validator app ignores it
- `CreateUser` (gRPC) — just a ledger user, validator app ignores it
- `topology/generate` + `topology/submit` — creates `PartyToParticipant` topology tx only, validator app ignores it completely

> **Reference:** <https://docs.dev.sync.global/scalability/scalability.html>

---

## The Dfns 100-Wallet Cap

Dfns enforces its own cap of **100 external-party wallets per validator**, separate from the Canton 200 limit. This applies to both shared and BYOV validators.

From Dfns's own BYOV announcement:
> *"Each validator can support up to 100 external-party wallets, as per Canton's current network recommendations."*
> *"Unlimited scaling within Canton's limits: Create up to 100 wallets per validator, and spin up more validators as needed."*

This cap exists because Dfns manages the MPC signing keys. Wallets created outside Dfns using your own Go code are not subject to this cap since Dfns is not involved.

> **Reference:** <https://www.dfns.co/article/canton-byov-support>

---

## Go Client Flow — Current vs Scalable

### Current Flow (hits caps at 200 / 100)

```bash
Step 1 — Create Auth0 user                      no cap
Step 2 — AllocateParty (gRPC)                   no cap
Step 3 — CreateUser (gRPC)                      no cap
Step 4 — OnboardWallet() → /register            ❌ creates WalletAppInstall → hits 200 cap
Step 5 — PreApproveTransfers()                  ❌ creates ValidatorRight   → hits 200 cap
         → /wallet/transfer-preapproval
```

### Scalable Flow (unlimited)

```bash
Step 1 — Create Auth0 user                      no cap  (unchanged)
Step 2 — AllocateParty (gRPC)                   no cap  (unchanged)
Step 3 — CreateUser (gRPC)                      no cap  (unchanged)
Step 4 — GenerateExternalPartyTopology()        ✅ no WalletAppInstall → bypasses 200
         → topology/generate + topology/submit
Step 5 — CreateTransferPreapprovalProposal()    ✅ no ValidatorRight   → bypasses 200
         → TransferPreapprovalProposal via Ledger API
```

Only **Steps 4 and 5** change. Everything else is identical.

---

## What You Lose With the Scalable Flow

| Feature | Current Flow | Scalable Flow |
| --- | --- | --- |
| Wallet visible in wallet UI | ✅ | ❌ |
| `/v0/admin/external-party/balance` | ✅ | ❌ |
| `/v0/admin/external-party/transfer-preapproval/submit-send` | ✅ | ❌ |
| Balance via Ledger API ACS query | ✅ | ✅ |
| Send / receive transfers | ✅ | ✅ |
| All gRPC Ledger API commands | ✅ | ✅ |
| All JSON Ledger API (`/v2/...`) | ✅ | ✅ |
| Transfer preapproval renewal automation | ✅ | ✅ (provider must = validator operator party) |

---

## Scalability Comparison

| Approach | Wallet Cap | Keys Held By |
| --- | --- | --- |
| Go client — current flow (`/register`) | 200 (Canton limit) | Auth0 / validator |
| Dfns BYOV | 100 (Dfns limit) | Dfns MPC |
| Dfns BYOV + multiple validators | 100 × N validators | Dfns MPC |
| Go client — scalable flow (`topology/generate`) | None (up to 1M) | You (AWS KMS recommended) |

---

## Key Reference Links

| Resource | URL |
| --- | --- | --- |
| Canton Scalability Docs (200 limit) | <https://docs.dev.sync.global/scalability/scalability.html> |
| Dfns Canton BYOV Announcement (100 cap) | <https://www.dfns.co/article/canton-byov-support> |
| Dfns Canton Network Docs | <https://docs.dfns.co/networks/canton> |
| Validator API External Signing | <https://docs.dev.sync.global/app_dev/validator_api/index.html#validator-api-external-signing> |
| Splice Validator Compose Docs | <https://docs.dev.sync.global/validator_operator/validator_compose.html> |
| Canton Network Overview | <https://docs.digitalasset.com/integrate/devnet/canton-network-overview/index.html> |
