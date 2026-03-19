# Canton Validator

This repository contains:

1. A running Canton / Splice validator deployed via Docker on an AWS EC2 instance.  
2. A Go client that programmatically interacts with the validator wallet APIs (transactions, balance, wallet creation, user onboarding, transaction pre-approval, active contract monitoring, etc.).  
3. SSH port-forwarding configuration for secure local access.  
4. JWT-based authentication for protected API access.  

---

## 1. Architecture Overview

## Components

- AWS EC2 instance running Ubuntu  
- Docker + docker-compose  
- Splice Validator container  
- Wallet API exposed internally on port 5003  
- External UI exposed via port 8080  
- Go client running locally on Mac  

### Flow

1. Validator runs inside Docker on EC2.  
2. Wallet API runs inside container (port 5003).  
3. We do NOT expose 5003 publicly.  
4. We use SSH port forwarding to securely access it from local machine.  
5. Go client connects to forwarded localhost port.  
6. All requests require a valid JWT token.  
7. Wallet creation, user onboarding, transaction pre-approval, and active contract monitoring are automated via the Go client.  

This ensures:

- No public exposure of wallet API  
- Authenticated access only  
- Controlled access through SSH  
- Automated wallet lifecycle and transaction management  

---

### Ledger API vs Wallet API (Important Distinction)

There are **two different APIs** exposed by the validator setup:

#### 1️⃣ Ledger API (gRPC – Port 5001)

- Accessed using `grpcurl`  
- Used for:  
  - Party creation  
  - User creation  
  - Granting act_as rights  
  - Submitting commands (e.g., WalletAppInstall)  
  - Querying active contracts  
  - Reading ledger updates  
- Service namespace:  
  `com.daml.ledger.api.v2.*`  

This is the **low-level Canton Ledger API**.

---

#### 2️⃣ Wallet API (REST – Port 5003)

- Accessed using HTTP requests  
- Base path:  
  `http://localhost:5003/api/validator/v0/`  
- Used for:  
  - Listing wallet transactions  
  - Getting wallet balance  
  - Wallet creation  
  - User onboarding  
  - Transaction pre-approval  
  - Active contract monitoring  

This is a **higher-level application API** built on top of the ledger.

---

Important:

- Port **5001 → Ledger (gRPC)**  
- Port **5003 → Wallet (REST)**  

They serve different purposes and require separate SSH port forwarding if accessed remotely.

---

## 5. SSH Port Forwarding (Security Layer)

We DO NOT expose port 5003 publicly.

Instead, we use:

```bash
ssh -i ~/Downloads/scopex_canton.pem \
    -L 5003:localhost:5003 \
    ubuntu@ec2-<public-ip>.compute.amazonaws.com
```

This creates:

Local machine → localhost:5003  
Tunnel → EC2 container:5003  

Meaning:

- Only your machine can access wallet API  
- No public attack surface  
- Production-safe approach  

### Generic Multi-Port SSH Tunnel Example

If you need access to multiple internal services (Ledger gRPC, Wallet API, Wallet UI, ANS UI), you can forward multiple ports in one command:

```bash
ssh -i ~/path/to/key.pem \
    -L 5001:localhost:5001 \
    -L 5003:localhost:5003 \
    -L 8080:wallet.localhost:80 \
    -L 8081:ans.localhost:80 \
    ubuntu@<EC2_PUBLIC_IP>
```

What this does:

- `-L 5001:localhost:5001` → Forwards Ledger gRPC API  
- `-L 5003:localhost:5003` → Forwards Wallet REST API  
- `-L 8080:wallet.localhost:80` → Forwards Wallet Web UI  
- `-L 8081:ans.localhost:80` → Forwards ANS Web UI  

Flow:

Local Machine → SSH Tunnel → EC2 → Docker Container → Internal Service  

This keeps all internal services private while allowing secure local access for development.

---

## 2. Docker Validator Setup

Validator runs inside:

```bash
canton-validator/splice-node/docker-compose/validator
```

Key commands used:

Start validator:

```bash
./start.sh -s "https://sv.sv-1.test.global.canton.network.sync.global" -o "<TESTNET_SECRET>" -p "scopex-validator-1" -m "1" -w
```

Important environment variables inside container:

```bash
SPLICE_APP_VALIDATOR_WALLET_USER_NAME=administrator
SPLICE_APP_VALIDATOR_LEDGER_API_AUTH_USER_NAME=ledger-api-user
```

This tells us which user the wallet API expects for authentication.

---

## 3. JWT Authentication

We generate token using:

```bash
python3 get-token.py administrator
```

Important:

- The username MUST match the wallet user inside container (`administrator`)  
- Otherwise: Authorization Failed  

JWT is then passed in header:

```bash
Authorization: Bearer <token>
```

---

## 4. Wallet API Endpoints Used

Base path (internal):

```bash
http://localhost:5003/api/validator/v0/
```

## List Transactions

POST request:

```bash
/wallet/transactions
```

Body:

```bash
{
  "page_size": 20
}
```

## Get Balance

GET request:

```bash
/wallet/balance
```

Response includes:

- effective_unlocked_qty  
- effective_locked_qty  
- round  
- total_holding_fees  

## Wallet Creation, User Onboarding, and Transaction Pre-Approval

These processes are fully automated via the Go client, enabling seamless wallet lifecycle management and pre-approval of transactions to streamline operations.

## Active Contract Monitoring

The Go client also monitors active contracts on the ledger, providing real-time insights into wallet holdings and contract states.

---

## 6. Financial Tracking and Limits

We have implemented financial tracking features to monitor traffic usage and enforce transfer limits:

- **Traffic Usage Monitoring:** Tracks API and transaction volume to ensure system performance and cost control.  
- **Transfer Limits:** Configurable limits on transaction amounts and frequencies to prevent abuse and maintain compliance.  

These features are integrated into the Go client and validator setup to provide robust financial oversight.

---

## 7. Go Client Implementation

Located in:

```bash
cmd/
```

Client responsibilities:

- Create HTTP client with timeout  
- Inject JWT into Authorization header  
- Call:  
  - ListTransactions(ctx, pageSize)  
  - GetBalance(ctx)  
  - Wallet creation and user onboarding workflows  
  - Transaction pre-approval  
  - Active contract monitoring  
- Print structured response  

Example main flow:

```bash
client, _ := cantonvalidator.NewCantonClient()

client.ListTransactions(ctx, 20)
client.GetBalance(ctx)
```

---

## 7. Security Model

Current Security:

- Wallet API not publicly exposed  
- JWT authentication required  
- Access only via SSH tunnel  
- Token tied to wallet user  

If additional security required:

- Rotate signing secret  
- Restrict EC2 security group to known IP  
- Disable public port 8080  
- Add reverse proxy with TLS  

---

## 8. What We Achieved

✔ Deployed Canton validator via Docker  
✔ Automated wallet creation and user onboarding  
✔ Automated transaction pre-approval and active contract monitoring  
✔ Implemented financial tracking for traffic and transfer limits  
✔ Secured API using SSH tunnel and JWT authentication  
✔ Built Go client for programmatic control  
✔ Fetched transactions and balance successfully  

System is now:

- Secure  
- Scriptable  
- Production-ready for automation  
- Extendable for transfers and validator control  

---

## 9. Next Possible Extensions

- Add transfer execution in Go  
- Add structured response parsing instead of map[string]interface{}  
- Add CLI flags  
- Add metrics  
- Add background sync loop  
- Add unit tests  
- Add structured logging  

---

## 10. Final Notes

Never expose port 5003 publicly.

Always use SSH tunnel or private networking.

JWT must match wallet user configured inside container.

This repository now serves as:

- Validator control client  
- Operational documentation  
- Security reference for deployment
