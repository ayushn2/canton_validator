# Canton Validator Node — Complete Fee Guide

## Environment

| Property | Value |
| --- | --- |
| Network | Testnet |
| Node | `scopex-validator-1` |
| Participant ID | `PAR::scopex-validator-1::12205f40...` |
| Uptime | 12+ days |
| Version | `3.4.11-snapshot.20260127.17528.0.ve786203c` |

---

## 1. Types of Fees

### 1.1 Traffic Fee

Charged when a transaction is submitted to the Global Synchronizer.

```bash
Fee = Transaction Size (bytes) × Traffic Rate (CC/MB)
```

### 1.2 Holding Fee (Demurrage)

Charged every round (~10 minutes) on CC sitting in any wallet.

```bash
Fee per round = CC Balance × 0.0000190259%

Example on 64,135 CC:
  Per round  →  ~0.0122 CC
  Per day    →  ~1.75 CC
  Per month  →  ~52.5 CC
```

---

## 2. Who Pays Traffic Fee

| Action | Traffic Fee? | Paid By |
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

> **Key Rule:** Traffic fee is always paid by the **originator** of the transaction — not by whoever's node physically submits it to the synchronizer.

---

## 3. Transaction Types Explained

### Type 1 — You Send CC From Your Wallet

```bash
You → Your Validator Node → Global Synchronizer
                                    ↑
                            YOUR wallet pays traffic fee
```

### Type 2 — User/App Wallet on Your Node Transacts

```bash
User Wallet → Your Validator Node → Global Synchronizer
                                            ↑
                                    THEIR wallet pays traffic fee
                                    YOUR node earns higher rewards
```

### Type 3 — Random Canton Transactions Routed Through Your Node

```bash
Other Participant → Your Validator Node → Global Synchronizer
                                                  ↑
                                          ORIGINATOR pays
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

## 4. Your Traffic State (Live Data)

Checked via Canton Console:

```scala
participant.traffic_control.traffic_state(
  participant.synchronizers.list_connected().head.synchronizerId
)
```

```scala
TrafficState(
  extraTrafficLimit     = 1,200,000 bytes   // 1.2 MB purchased
  extraTrafficConsumed  = 0 bytes           // nothing used yet
  baseTrafficRemainder  = 200,000 bytes     // 200KB free allowance
  lastConsumedCost      = 0                 // last txn cost = 0
  availableTraffic      = 1,400,000 bytes   // 1.4 MB total available
)
```

### Traffic Buckets

| Bucket | Size | Cost | Replenishes? |
| --- | --- | --- | --- |
| Base Traffic | 200,000 bytes | FREE | ✅ Every round |
| Extra Traffic | 1,200,000 bytes | CC | ❌ Must top up |
| **Total Available** | **1,400,000 bytes** | | |

---

## 5. Your Complete Fee Picture (Right Now)

| Fee Type | Amount | Notes |
| --- | --- | --- |
| Traffic fees paid | **0 CC** | Nothing consumed yet |
| Extra traffic consumed | **0 / 1,200,000 bytes** | Full balance available |
| Base traffic remaining | **200,000 bytes** | Replenishes every round for free |
| Holding fee per round | **~0.0122 CC** | On 64,135 CC balance |
| Holding fee per day | **~1.75 CC** | Ongoing |
| Holding fee per month | **~52.5 CC** | Ongoing |

---

## 6. Your Earnings vs Costs

| Item | Per Round | Per Day | Per Month |
| --- | --- | --- | --- |
| Validator Rewards | +33.1081975897 CC | ~+4,767 CC | ~+142,000 CC |
| Holding Fee | -0.0122 CC | ~-1.75 CC | ~-52.5 CC |
| Traffic Fee | 0 CC | 0 CC | 0 CC |
| **Net** | **+33.096 CC** | **~+4,765 CC** | **~+141,947 CC** |

> You are keeping **99.96%** of your earnings. Fees are negligible.

---

## 7. How to Check Traffic Fee via Canton Console

### Step 1 — Access Canton Console

```bash
# Create remote config
docker exec -it splice-validator-participant-1 bash -c 'cat > /tmp/remote.conf << EOF
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
EOF'

# Launch console
docker exec -it splice-validator-participant-1 \
  /app/bin/canton --config /tmp/remote.conf
```

### Step 2 — Check Node Health

```scala
participant.health.status
```

### Step 3 — Check Connected Synchronizers

```scala
participant.synchronizers.list_connected()
```

### Step 4 — Check Traffic State

```scala
participant.traffic_control.traffic_state(
  participant.synchronizers.list_connected().head.synchronizerId
)
```

### Step 5 — Check Your Parties

```scala
participant.parties.list()
```

---

## 8. Key Takeaways

- **Simply running your node = near zero cost in CC**
- **Traffic fee only triggers when YOUR wallet or wallets YOU OWN actively submit transactions**
- **Every wallet pays its own traffic fee independently**
- **Base traffic (200KB) replenishes every round for free — simple transactions are covered**
- **Your validator rewards (~33 CC/round) far exceed your total fees**
- **Your only real cost is server infrastructure in USD (~$50–150/month on testnet)**
