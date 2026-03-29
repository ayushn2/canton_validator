# Canton Validator Node — Complete Fee Guide

## Environment

| Property | Value |
| --- | --- |
| Network | Testnet |
| Node | `scopex-validator-1` |
| Participant ID | `PAR::scopex-validator-1::12205f40...` |
| Uptime | 12+ days |
| Version | `3.4.11-snapshot.20260127.17528.0.ve786203c` |
| Synchronizer | `global-domain::1220f22a8b8f2d813c25b9a684dc4dd52b532a0174d8e73a13cdf2baabfff7518337` |

---

## 1. DSO Contract Parameters (Live — Verified)

> Source: queried directly from `https://scan.sv-1.test.global.canton.network.sync.global/api/scan/v0/amulet-rules`

```bash
curl -s -X POST \
  https://scan.sv-1.test.global.canton.network.sync.global/api/scan/v0/amulet-rules \
  -H "Content-Type: application/json" \
  -d '{}' \
  | python3 -m json.tool
```

| Parameter | Value | Field in DSO Contract |
| --- | --- | --- |
| Base traffic per window | 200,000 bytes (200 KB) | `burstAmount` |
| Base traffic window | 20 minutes (1,200,000,000 microseconds) | `burstWindow.microseconds` |
| Extra traffic price | **60.0 CC per MB** | `extraTrafficPrice` |
| Min top-up amount | 200,000 bytes | `minTopupAmount` |
| Holding fee rate | 0.0000190259% per round | `holdingFee.rate` |
| Transfer fee | 0.0 CC | `transferFee.initialRate` |
| Create fee | 0.0 CC | `createFee.fee` |
| Validator reward % | 5% of total issuance | `validatorRewardPercentage` |
| Validator reward cap | 0.2 CC per round | `validatorRewardCap` |
| Validator faucet cap | 570.0 CC | `optValidatorFaucetCap` |
| CC issued per year | 40,000,000,000 CC | `amuletToIssuePerYear` |

---

## 2. Types of Fees

### 2.1 Traffic Cost

Every transaction consumes bytes from your traffic allowance when submitted to the Global Synchronizer.

```bash
Traffic consumed = Transaction Size (bytes)

If within base traffic  →  FREE
If base exhausted       →  Extra traffic used = costs CC

Cost = Extra Bytes Consumed (MB) × 60 CC/MB
```

> ⚠️ There is no separate transfer fee or create fee on this network (both are 0.0 CC). Traffic consumption is the only transaction-level cost.

### 2.2 Holding Fee (Demurrage)

Charged every round (~20 minutes) on CC sitting in any wallet.

```bash
Fee per round = CC Balance × 0.0000190259%

Example on 64,135 CC:
  Per round  →  ~0.0122 CC
  Per day    →  ~0.87 CC   (72 rounds/day)
  Per month  →  ~26.25 CC
```

---

## 3. Who Pays Traffic Cost

| Action | Traffic Consumed? | Paid By |
| --- | --- | --- |
| You send CC from your wallet | ✅ YES | Your wallet |
| You create a wallet | ✅ YES | Your wallet |
| You deploy a Daml contract | ✅ YES | Your wallet |
| You exercise a contract choice | ✅ YES | Your wallet |
| User wallet on your node transacts | ✅ YES | Their wallet |
| Node running idle | ❌ NO | — |
| Receiving CC | ❌ NO | — |
| Protocol auto transactions | ❌ NO | Protocol layer |
| Random Canton txns routed through your node | ❌ NO | Originator pays |
| Read-only console commands | ❌ NO | — |

> **Key Rule:** Traffic cost is always paid by the **originator** of the transaction — not by whoever's node physically submits it to the synchronizer.

---

## 4. Transaction Types Explained

### Type 1 — You Send CC From Your Wallet

```bash
You → Your Validator Node → Global Synchronizer
                                    ↑
                            YOUR wallet consumes traffic
```

### Type 2 — User/App Wallet on Your Node Transacts

```bash
User Wallet → Your Validator Node → Global Synchronizer
                                            ↑
                                    THEIR wallet consumes traffic
                                    YOUR node earns higher rewards
```

### Type 3 — Random Canton Transactions Routed Through Your Node

```bash
Other Participant → Your Validator Node → Global Synchronizer
                                                  ↑
                                          ORIGINATOR consumes traffic
                                          YOU pay nothing
                                          YOUR activity score goes up → more rewards
```

### Type 4 — Protocol Auto Transactions

```bash
DSO Smart Contract → Your Validator Node → Global Synchronizer
                                                   ↑
                                           PROTOCOL pays
                                           YOU pay nothing
                                           YOU receive validator rewards
```

---

## 5. Transaction Sizes (Measured Live)

> Measured by comparing `traffic_control.traffic_state` before and after each transaction on the live node.

### How We Measured

```scala
// Step 1: Record before state
val before = participant.traffic_control.traffic_state(
  participant.synchronizers.list_connected().head.synchronizerId
)
println(s"Before - Base: ${before.baseTrafficRemainder}, Last Cost: ${before.lastConsumedCost}")

// Step 2: Submit transaction (wallet creation or transfer)

// Step 3: Record after state
val after = participant.traffic_control.traffic_state(
  participant.synchronizers.list_connected().head.synchronizerId
)
println(s"After - Base: ${after.baseTrafficRemainder}, Last Cost: ${after.lastConsumedCost}")

// lastConsumedCost = exact bytes of last transaction
// Difference in baseTrafficRemainder = total bytes consumed in window
```

### Results

| Transaction Type | Bytes Consumed | KB | Steps Involved |
| --- | --- | --- | --- |
| Single step (party alloc / user create) | ~2,970 bytes | ~2.97 KB | 1 |
| Full wallet creation | ~13,500 bytes | ~13.5 KB | 4 steps |
| CC Transfer | ~8,070 bytes | ~8.07 KB | 1 |

### Full Wallet Creation — 4 Steps Breakdown

```bash
make wallet
# runs 4 transactions:
#   1. Allocate Party        → ~2,970 bytes
#   2. Create User           → ~2,970 bytes
#   3. Onboard Wallet        → ~3,780 bytes (estimated)
#   4. Pre-approve Transfer  → ~3,780 bytes (estimated)
#   ─────────────────────────────────────────
#   Total                    → ~13,500 bytes
```

### Cost If Extra Traffic Needs To Be Topped Up (60 CC/MB)

| Transaction | Bytes | MB | Cost in CC |
| --- | --- | --- | --- |
| Full wallet creation | 13,500 | 0.0135 MB | **0.81 CC** |
| CC Transfer | 8,070 | 0.00807 MB | **0.4842 CC** |
| Single step | 2,970 | 0.00297 MB | **0.1782 CC** |

> ⚠️ CC is only spent when you **purchase** extra traffic — not when you use it. The cost above reflects what you pay per transaction worth of extra traffic when topping up.
>
> The full consumption order before any CC is spent:
>
> 1. Base traffic used first (200KB FREE — resets every 20 min)
> 2. Extra traffic used next (1,200,000 bytes already available — no CC needed yet)
> 3. Extra traffic exhausted → **only now do you spend CC to top up** (minimum 12 CC = 200,000 bytes)

---

## 6. Traffic State (Live Data)

### How To Check

```bash
# Step 1: Enter participant container
docker exec -it splice-validator-participant-1 bash

# Step 2: Create remote console config
cat > /tmp/remote.conf << EOF
canton {
  remote-participants {
    participant {
      admin-api {
        address = "127.0.0.1"
        port = 5002
      }
      ledger-api {
        address = "127.0.0.1"
        port = 5001
      }
    }
  }
}
EOF

# Step 3: Launch Canton Console
/app/bin/canton --config /tmp/remote.conf
```

```scala
// Step 4: Check traffic state inside console
participant.traffic_control.traffic_state(
  participant.synchronizers.list_connected().head.synchronizerId
)
```

### Your Live Traffic State

```bash
TrafficState(
  extraTrafficLimit     = 1,200,000 bytes   // 1.2 MB purchased with CC
  extraTrafficConsumed  = 0 bytes           // nothing used yet
  baseTrafficRemainder  = 200,000 bytes     // 200KB free — resets every 10 min
  lastConsumedCost      = 0                 // last txn consumed 0 bytes
  availableTraffic      = 1,400,000 bytes   // total available right now
)
```

### Traffic Buckets

| Bucket | Size | Cost | Replenishes? |
| --- | --- | --- | --- |
| Base Traffic | 200,000 bytes | FREE | ✅ Every 20 minutes |
| Extra Traffic | 1,200,000 bytes | 60 CC/MB | ❌ Manual top-up with CC |
| **Total Available** | **1,400,000 bytes** | | |

### Minimum Top-Up Cost

```bash
Min top-up = 200,000 bytes = 0.2 MB
Cost       = 0.2 × 60 CC  = 12 CC minimum per top-up
```

---

## 7. Free Transactions Per Round (Base Traffic Only)

| Transaction Type | Size | Free Per Round | Free Per Day (72 rounds) |
| --- | --- | --- | --- |
| Full wallet creation | 13,500 bytes | ~14 wallets | ~1,008 wallets |
| CC Transfer | 8,070 bytes | ~24 transfers | ~1,728 transfers |
| Single step | 2,970 bytes | ~67 operations | ~4,824 operations |

---

## 8. Complete Fee Picture (Right Now)

### How To Check Node Health

```scala
// Inside Canton Console
participant.health.status
participant.synchronizers.list_connected()
```

| Fee Type | Amount | Notes |
| --- | --- | --- |
| Traffic fees paid | **0 CC** | Nothing consumed yet |
| Extra traffic consumed | **0 / 1,200,000 bytes** | Full balance available |
| Base traffic remaining | **200,000 bytes** | Resets every 20 min for free |
| Holding fee per round | **~0.0122 CC** | On 64,135 CC balance |
| Holding fee per day | **~0.87 CC** | 72 rounds/day |
| Holding fee per month | **~26.25 CC** | Ongoing |

---

## 9. Earnings vs Costs

| Item | Per Round | Per Day | Per Month |
| --- | --- | --- | --- |
| Validator Rewards | +33.1081975897 CC | ~+2,384 CC | ~+71,500 CC |
| Holding Fee | -0.0122 CC | ~-0.87 CC | ~-26.25 CC |
| Traffic Cost | 0 CC | 0 CC | 0 CC |
| **Net** | **+33.096 CC** | **~+2,383 CC** | **~+71,474 CC** |

> You are keeping **99.96%** of your earnings. Fees are negligible.

---

## 10. Issuance Schedule (From DSO Contract)

| Period | CC Issued/Year | Validator % | Validator Cap | Starts After |
| --- | --- | --- | --- | --- |
| Current | 40,000,000,000 CC | 5% | 0.2 CC/round | Now |
| Period 2 | 20,000,000,000 CC | 12% | 0.2 CC/round | ~6 months |
| Period 3 | 10,000,000,000 CC | 18% | 0.2 CC/round | ~18 months |
| Period 4 | 5,000,000,000 CC | 21% | 0.2 CC/round | ~50 years |
| Period 5 | 2,500,000,000 CC | 20% | 0.2 CC/round | ~100 years |

> Issuance halves over time but validator percentage increases — designed to maintain validator incentives long term.

---

## 11. Key Takeaways

- **Simply running your node = near zero cost in CC**
- **Traffic cost only triggers when YOUR wallet or wallets YOU OWN actively submit transactions**
- **Every wallet pays its own traffic cost independently from its own CC balance**
- **There is no transfer fee or create fee — traffic consumption is the only transaction-level cost**
- **Base traffic (200KB) resets every 20 minutes for free — normal operations are fully covered**
- **Extra traffic costs 60 CC per MB and must be manually topped up (minimum 12 CC per top-up)**
- **Your validator rewards (~33 CC/round) far exceed your total fees**
- **Your only real monetary cost is server infrastructure in USD (~$50–150/month)**
