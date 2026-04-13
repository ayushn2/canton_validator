# Canton Validator

A Go client library and CLI toolkit for programmatically interacting with a Canton/Splice validator node. Covers the full wallet lifecycle — creation, onboarding, transfers, active contract queries — with dual-mode authentication (Auth0 for production, unsafe JWT for local dev).

---

## What This Repo Does

1. **Go client library** (`cantonvalidator/`) — wraps both the Canton Ledger gRPC API and the Validator REST API.
2. **CLI commands** (`cmd/`) — runnable via `make` for wallet creation, CC transfers, and ledger inspection.
3. **Auth0 integration** — programmatic Auth0 user creation, M2M token caching, and resource-owner password grant for per-wallet tokens.
4. **Topology-based scaling** — bypasses the 200-party automation cap by using Canton topology transactions for external signing, replacing in-process wallet creation.
5. **Local wallet store** (`db/`) — persists wallet credentials (Auth0 user, Canton user, party ID) to `wallets/wallets.json`.
6. **Operational documentation** (`docs/`) — covers authentication, fee structure, scalability limits, USDCx integration, and gRPC command reference.

---

## Architecture

```text
Local Mac
  └── Go Client
        ├── gRPC → localhost:5001 ──(SSH tunnel)──► EC2 → Docker → Canton Ledger API
        └── REST → localhost:5003 ──(SSH tunnel)──► EC2 → Docker → Validator Wallet API
```

The validator runs inside Docker on an AWS EC2 instance. Neither port 5001 nor 5003 is publicly exposed — access is exclusively via SSH port forwarding.

### Two APIs

| API | Protocol | Port | Used For |
| --- | --- | --- | --- |
| Canton Ledger API | gRPC | 5001 | Party allocation, user creation, act_as grants, ACS queries |
| Validator Wallet API | REST | 5003 | Wallet onboarding, transfer pre-approval, CC transfers, balance |

---

## Directory Structure

```text
canton_validator/
├── cantonvalidator/          # Core client library
│   ├── auth.go               # Dual-mode JWT (unsafe HS256 + Auth0 RS256)
│   ├── canton_client.go      # gRPC wrapper (party grants, ledger polling)
│   ├── wallet_service.go     # Full wallet lifecycle orchestration
│   ├── transfer_service.go   # CC transfer execution
│   └── ledger.go             # ACS queries, wallet listing
├── cmd/
│   ├── wallet/main.go        # `make wallet` — create a wallet
│   ├── transfer/main.go      # `make transfer` — send CC between wallets
│   └── ledger/main.go        # `make ledger` — list all wallets on ledger
├── config/
│   ├── config.go             # Loads config/.env
│   └── .env.example          # Environment variable template
├── db/
│   └── wallet_store.go       # JSON-backed wallet credential store
├── docs/
│   ├── authentication.md     # Auth0 setup and token types
│   ├── create-wallet.md      # Manual wallet creation walkthrough
│   ├── commands.md           # gRPC/CLI command reference
│   ├── canton_financials.md  # Fee structure and traffic costs
│   ├── canton_wallet_scaling.md  # 200-party cap and scaling solutions
│   └── usdcx_integration.md  # USDC on Canton Network (11-step guide)
├── Makefile
├── go.mod
└── go.sum
```

---

## Setup

### Prerequisites

- Go 1.21+
- `grpcurl` (for manual gRPC inspection)
- An Auth0 tenant with an M2M application and a user database connection
- SSH access to the EC2 validator instance

### Install

```bash
git clone <repo>
cd canton_validator
go mod download
cp config/.env.example config/.env
# Fill in config/.env — see Environment Variables section below
```

### SSH Tunnel

Before running any commands, forward the Canton ports from EC2:

```bash
ssh -i ~/path/to/key.pem \
    -L 5001:localhost:5001 \
    -L 5003:localhost:5003 \
    ubuntu@<EC2_PUBLIC_IP>
```

Optional — also forward the web UIs:

```bash
ssh -i ~/path/to/key.pem \
    -L 5001:localhost:5001 \
    -L 5003:localhost:5003 \
    -L 8080:wallet.localhost:80 \
    -L 8081:ans.localhost:80 \
    ubuntu@<EC2_PUBLIC_IP>
```

---

## Environment Variables

Copy `config/.env.example` to `config/.env` and fill in:

| Variable | Description |
| --- | --- |
| `GRPC_HOST` | Canton Ledger gRPC address (`localhost:5001`) |
| `VALIDATOR_URL` | Wallet REST base URL (`http://localhost:5003`) |
| `VALIDATOR_PARTY` | Validator operator party ID |
| `DSO_PARTY` | DSO party ID |
| `ADMIN_TOKEN` | Admin JWT for ledger operations (unsafe mode) |
| `TESTNET_SECRET` | Canton testnet signing secret |
| `JWT_SECRET` | Shared secret for unsafe JWT signing (dev only) |
| `JWT_AUDIENCE` | JWT audience claim |
| `AUTH0_DOMAIN` | Auth0 tenant domain |
| `VALIDATOR_AUTH_CLIENT_ID` | Auth0 M2M client ID |
| `VALIDATOR_AUTH_CLIENT_SECRET` | Auth0 M2M client secret |
| `VALIDATOR_AUTH_AUDIENCE` | Auth0 API audience |
| `LEDGER_API_ADMIN_USER` | Ledger API admin user name |
| `TEST_WALLET_PASSWORD` | Default password for test wallets |

---

## Usage

```bash
make wallet     # Create a new wallet (Auth0 user → Canton party → onboard → pre-approve)
make transfer   # Transfer CC between two wallets
make ledger     # List all users and party IDs on the ledger
make all        # wallet + transfer
make test       # Run tests
make clean      # Clear build cache
```

---

## Core Features

### Wallet Creation (`cantonvalidator/wallet_service.go`)

`CreateWallet()` orchestrates five steps:

1. Create an Auth0 user (email + password) via the Management API
2. Allocate a Canton party via `PartyManagementService/AllocateParty` (gRPC)
3. Create a Daml ledger user with `act_as` and `read_as` rights (gRPC)
4. Onboard the wallet via `POST /api/validator/v0/register` (REST)
5. Pre-approve incoming transfers via `POST /api/validator/v0/wallet/transfer-preapproval` (REST)

### Topology-Based Scaling (`docs/canton_wallet_scaling.md`)

The default Canton validator automation limit is **200 parties**. To go beyond this, the client uses topology transactions for external signing — wallet creation is handled outside the validator's automation stack, bypassing the cap. See [docs/canton_wallet_scaling.md](docs/canton_wallet_scaling.md) for details.

### Authentication (`cantonvalidator/auth.go`)

Two modes controlled by config:

| Mode | Algorithm | Use Case |
| --- | --- | --- |
| Unsafe | HS256 with shared secret | Local dev and testnet |
| Auth0 | RS256 via Auth0 M2M | Production |

Auth0 tokens are cached with a 60-second refresh margin to avoid redundant calls.

### CC Transfers (`cantonvalidator/transfer_service.go`)

`Transfer()` sends Canton Coin between wallets:

- Uses per-wallet resource-owner tokens (not the admin token)
- Generates a UUID tracking ID per transfer
- 24-hour transfer expiry
- Endpoint: `POST /api/validator/v0/wallet/token-standard/transfers`

### Ledger Queries (`cantonvalidator/ledger.go`)

- `GetActiveContracts()` — queries the Active Contract Set (ACS) by party via gRPC
- `WalletAlreadyInstalled()` — checks for duplicate `WalletAppInstall` contracts
- `GetAllWallets()` — lists all users and their primary party IDs

### Local Wallet Store (`db/wallet_store.go`)

Credentials are persisted to `wallets/wallets.json` (git-ignored). Each entry stores:

```json
{
  "name": "...",
  "email": "...",
  "password": "...",
  "auth0_user_id": "...",
  "canton_user_id": "...",
  "party_id": "..."
}
```

---

## Documentation

| File | Contents |
| --- | --- |
| [docs/authentication.md](docs/authentication.md) | Auth0 tenant setup, token types (M2M, user, management), testnet config |
| [docs/create-wallet.md](docs/create-wallet.md) | Manual 3-step wallet creation walkthrough |
| [docs/commands.md](docs/commands.md) | Validator startup, Canton console, traffic monitoring, pre-approval gRPC calls |
| [docs/canton_financials.md](docs/canton_financials.md) | DSO parameters, traffic costs (60 CC/MB), holding fees, transaction sizes |
| [docs/canton_wallet_scaling.md](docs/canton_wallet_scaling.md) | 200-party cap, 100-wallet Dfns limit, topology-based scaling approach |
| [docs/usdcx_integration.md](docs/usdcx_integration.md) | End-to-end USDC on Canton: onboarding, minting USDCx, burning back to Ethereum |

---

## Security Model

- Ports 5001 and 5003 are **not publicly exposed** — accessible only via SSH tunnel
- JWT authentication required on all requests
- Auth0 M2M tokens scoped to validator API audience
- Wallet credentials stored locally and never committed (`.gitignore`)
- Unsafe JWT mode is for development only; Auth0 mode is required for production

---

## What's Implemented

- Canton validator deployment via Docker on EC2
- Full programmatic wallet lifecycle (Auth0 user → Canton party → onboard → pre-approve)
- CC transfer execution between wallets
- Active contract set (ACS) querying via gRPC
- Ledger user and wallet listing
- Dual-mode authentication (unsafe + Auth0) with token caching
- Topology-based wallet creation to bypass the 200-party cap
- Local JSON wallet credential store
- USDCx (USDC on Canton) integration guide
- Financial tracking documentation (fees, traffic costs, holding fees)
