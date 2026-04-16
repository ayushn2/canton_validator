# Canton Featured App — Glossary of Terms

## The Core Concepts

### FeaturedAppRight

**What it is:** A Daml contract that lives on your ledger. It's essentially a permission slip
from the Canton Foundation saying "this app is approved to earn Featured App rewards."

**Who creates it:** The Super Validators, after the Tokenomics Committee approves your application
through a ⅔ majority vote.

**Why you need it:** Without this contract on your ledger, you cannot create activity markers,
and your app cannot earn from the 516M CC monthly reward pool. It's the difference between
earning ~$2,400/month (validator liveness only) and earning potentially $100K+/month.

**On devnet:** You can grant this to yourself through the wallet UI (self-feature).
**On mainnet:** You must apply at canton.foundation/featured-app-request and wait for approval.

---

### FeaturedAppActivityMarker

**What it is:** A Daml contract that gets created inside a transaction to say "a real economic
event just happened through this Featured App."

**What it contains:**

- provider: your validator party
- beneficiary: who gets the minting right (usually same as provider)
- weight: set to $1 by default (the featuredAppActivityMarkerAmount)

**What happens to it:** Super Validator automation picks it up, converts it to an AppRewardCoupon,
and your validator auto-mints CC from it. You don't do anything — it's automatic.

**Current status:** This is the live mechanism today. CIP-0104 will eventually replace it with
traffic-based rewards, but markers are still active on Splice 0.5.12.

---

### WalletUserProxy

**What it is:** A ready-made Daml template provided by Splice (in the
splice-util-featured-app-proxies.dar package) specifically designed for wallet providers like you.

**What it does:** Two things in one:

1. Wraps a standard transfer with a FeaturedAppActivityMarker (earns rewards today)
2. Makes your validator party a stakeholder in the transaction (ensures traffic attribution
   under both the current marker system and the future traffic-based system)

**How you use it:** Instead of calling `TransferFactory_Transfer` for a user's transfer,
you call `WalletUserProxy_TransferFactory_Transfer`. Same transfer happens, but now it
earns rewards.

**Why it exists:** Splice built this so wallet providers don't have to write custom Daml code.
It's the officially recommended approach.

---

### AppRewardCoupon

**What it is:** A Daml contract that represents your right to mint a specific amount of CC.

**Where it comes from:** Super Validator automation converts your FeaturedAppActivityMarker
into an AppRewardCoupon every round (every 10 minutes).

**What happens to it:** Your validator's background automation automatically mints the CC
from the coupon into your balance. You don't manually claim anything.

**If not claimed:** Coupons expire if not minted in time — "All rewards and coupons are
mintable the following mining round, if rewards are not redeemed then they are lost."

---

## The Infrastructure Terms

### Party

**What it is:** A unique identity on Canton. Every user has one, and your validator has one.

**Examples:**

- Your validator party: `name-validator-1::12205f40f735c6d338ec14f0bcebe8de5c43f670ec9bb2666ede81806353a30a394c`
- A user party: `user-123::12205f40...`

**Why it matters:** The reward system attributes rewards to parties, not to nodes. Your
validator party must be involved in a transaction for you to earn rewards from it. Just
hosting a user's party on your node is not enough.

---

### App Provider Party

**What it is:** Your validator's party ID when it's registered as a Featured App. This is
the party that the FeaturedAppRight is granted to, and the party that earns minting rights.

---

### Participant Node

**What it is:** The Canton software that runs on your EC2 instance. It hosts parties,
processes transactions, stores the ledger, and communicates with the Global Synchronizer.

**Your participant:** The `splice-validator-participant-1` Docker container on your EC2.

---

### Global Synchronizer

**What it is:** The decentralized coordination layer operated by 26+ Super Validators.
It ensures all transactions across Canton are ordered, validated, and finalized. Every
transaction on Canton goes through it.

**Why it costs traffic:** Using the Global Synchronizer requires bandwidth (traffic).
Traffic is bought with CC and is the main operating cost for running apps.

---

### Ledger API

**What it is:** The interface your applications use to interact with your participant node.
You submit transactions, query contracts, and upload packages through it.

**Your Ledger API:** Accessible at `https://<url>/ledger/v2/...` (through nginx)
or at `localhost:7575` inside the participant container.

**Authentication:** Requires an Auth0 M2M token (using VALIDATOR_CLIENT_ID and
VALIDATOR_CLIENT_SECRET with AUTH_AUDIENCE).

---

## The Tokenomics Terms

### Round (Mining Round)

**What it is:** A 10-minute cycle during which activity records are collected, rewards
are calculated, and CC can be minted.

**What happens each round:**

1. Activity recording phase — markers and other activity records are created
2. Calculation phase — CC-per-weight is determined for each type of activity
3. Minting phase — parties mint their earned CC from coupons

---

### Activity Record

**What it is:** A record that says "this party did something valuable this round."
Activity records have a weight, which determines your share of the reward pool.

**Types:**

- ValidatorRewardCoupon — earned for validator liveness (~4 CC/round for you)
- AppRewardCoupon — earned from featured app activity (the big rewards)
- FeaturedAppActivityMarker — created by your app, converted to AppRewardCoupon by SV automation

---

### Minting Weight

**What it is:** The numerical value on your activity record that determines how much CC
you can mint relative to other apps.

**Current value per marker:** $1 (the featuredAppActivityMarkerAmount parameter)

**How it works:** Your weight / total weight of all apps × CC available for apps this round
= CC you can mint.

---

### Traffic / Traffic Fees

**What it is:** The bandwidth cost of putting transactions on the Global Synchronizer.
Every transaction consumes some amount of traffic (measured in bytes/MB).

**How you pay:** You buy traffic using CC. This is your operating cost.

**Under CIP-0104:** Traffic becomes the basis for reward calculation instead of markers.
More traffic your app consumes = bigger share of rewards.

---

### Beneficiary

**What it is:** The party that actually gets to mint the CC from a reward coupon.

**Why it matters:** When creating a marker, you can split the reward between multiple
beneficiaries with weights that sum to 1.0. For example:

- 100% to your validator party (you keep everything)
- 90% to you, 10% to the user (user gets CC cashback)

---

## The Package Terms

### DAR file

**What it is:** A compiled Daml package file (Daml ARchive). Contains Daml templates
and their dependencies. Like a .jar file in Java.

**Relevant DARs for your Featured App:**

- `splice-api-featured-app-v1-1.0.0.dar` — the FeaturedAppRight interface (already on your node)
- `splice-util-featured-app-proxies-1.2.1.dar` — the WalletUserProxy template (uploaded by us)

---

### CIP-56 (Canton Token Standard)

**What it is:** The standard for how tokens (CC, USDCx, USDXLR, etc.) are represented
and transferred on Canton. Defines interfaces like TransferFactory, Holding, etc.

**Why it matters:** The WalletUserProxy wraps the CIP-56 TransferFactory_Transfer choice
with marker creation. Your Dfns BYOV transfers already use CIP-56.

---

### CIP-47 (Featured App Activity Markers)

**What it is:** The CIP that introduced FeaturedAppActivityMarkers — allowing apps to earn
rewards without needing to do CC transfers specifically.

**Status:** Live and active on your Splice 0.5.12 node.

---

### CIP-0104 (Traffic-Based App Rewards)

**What it is:** The approved CIP that will eventually replace marker-based rewards with
traffic-based rewards.

**Current status:** Only Increment 1 is live (free confirmation responses, since March 2nd).
Markers are still the active reward mechanism. Full transition (Increment 4) hasn't happened yet
and requires at least 30 days after Increment 2 lands on mainnet.

**Impact on you:** When fully rolled out, you won't need markers anymore. But you still need
the WalletUserProxy to make your party a stakeholder in transactions for traffic attribution.

---

## The Key Relationship

```bash
You apply for Featured App status
    → Tokenomics Committee approves
    → FeaturedAppRight contract appears on your ledger
    → You create WalletUserProxy contracts for your users
    → Users do transfers via WalletUserProxy_TransferFactory_Transfer
    → Each transfer creates a FeaturedAppActivityMarker
    → SV automation converts marker to AppRewardCoupon
    → Your validator auto-mints CC from coupon
    → CC appears in your balance
    → Repeat every transfer, every round, every day
```
