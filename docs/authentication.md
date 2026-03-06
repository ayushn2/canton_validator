# Authentication

This document covers the full authentication setup for the Canton Validator Go client, including local development (unsafe JWT) and production (Auth0).

---

## Overview

The project supports two authentication modes controlled by the `AUTH_MODE` environment variable:

| Mode | When to Use | Token Type |
| --- | --- | --- |
| `unsafe` | Local development only | HS256 JWT with shared secret |
| `auth0` | Testnet / Mainnet | RS256 JWT via Auth0 |

---

## Auth Flow Diagram

```bash
┌─────────────────────────────────────────────────────────┐
│                    Go Client                            │
│                                                         │
│  GenerateToken()  ──► unsafe  ──► HS256 JWT             │
│       │                                                 │
│       └──────────► auth0   ──► Auth0 M2M Token          │
│                                                         │
│  GetUserToken()   ──────────► Auth0 Password Grant      │
│                               (wallet operations)       │
└─────────────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────┐      ┌──────────────────────┐
│ Participant Node│      │   Validator Node      │
│ (admin ops)     │      │ (wallet/transfer ops) │
│ gRPC :5001      │      │ HTTP :5003            │
└─────────────────┘      └──────────────────────┘
```

---

## Auth0 Setup

### Tenant

| Field | Value |
| --- | --- |
| Tenant Name | `dev-h6x5kvkxhspo860c` |
| Domain | `dev-h6x5kvkxhspo860c.us.auth0.com` |
| Region | US |

---

### APIs

#### Canton Validator API

| Field | Value |
| --- | --- |
| Name | `Canton Validator API` |
| Identifier (Audience) | `https://canton-test.scopex.app` |
| Signing Algorithm | `RS256` |

#### Auth0 Management API

| Field | Value |
| --- | --- |
| Identifier | `https://dev-h6x5kvkxhspo860c.us.auth0.com/api/v2/` |
| Used for | Creating Auth0 users programmatically |

---

### Applications

#### ScopeX Canton Test (M2M App)

| Field | Value |
| --- | --- |
| Type | Machine to Machine |
| Grant Types | Client Credentials, Password |
| Authorized APIs | Canton Validator API, Auth0 Management API |
| Management API Scopes | `create:users`, `read:users`, `update:users` |

Used for:

- Admin operations (party allocation, user creation)
- Fetching Management API tokens to create Auth0 users programmatically

#### Canton Wallet UI (SPA)

| Field | Value |
| --- | --- |
| Type | Single Page Application |
| Allowed Callback URLs | `http://localhost:8080` |
| Allowed Logout URLs | `http://localhost:8080` |
| Allowed Web Origins | `http://localhost:8080` |

#### Canton ANS UI (SPA)

| Field | Value |
| --- | --- |
| Type | Single Page Application |
| Allowed Callback URLs | `http://localhost:8080` |
| Allowed Logout URLs | `http://localhost:8080` |
| Allowed Web Origins | `http://localhost:8080` |

---

### Tenant Settings

**API Authorization Settings** (Settings → General → Advanced):

| Field | Value |
| --- | --- |
| Default Audience | `https://canton-test.scopex.app` |
| Default Directory | `Username-Password-Authentication` |

The Default Directory must be set for the Resource Owner Password Grant to work.

---

## Environment Variables

### Local Go Client (`config/.env`)

```dotenv
# Auth mode
AUTH_MODE=auth0

# Auth0 M2M credentials
AUTH0_DOMAIN=dev-h6x5kvkxhspo860c.us.auth0.com
VALIDATOR_AUTH_CLIENT_ID=<m2m-client-id>
VALIDATOR_AUTH_CLIENT_SECRET=<m2m-client-secret>
VALIDATOR_AUTH_AUDIENCE=https://canton-test.scopex.app

# Default wallet user (used as fallback)
AUTH0_WALLET_USERNAME=ayush.nainwal@scopex.money
AUTH0_WALLET_PASSWORD=<password>

# Ledger API admin user
LEDGER_API_ADMIN_USER=<m2m-client-id>@clients
```

### EC2 Validator Node (`.env`)

```dotenv
# Auth0 endpoints
AUTH_URL=https://dev-h6x5kvkxhspo860c.us.auth0.com
AUTH_JWKS_URL=https://dev-h6x5kvkxhspo860c.us.auth0.com/.well-known/jwks.json
AUTH_WELLKNOWN_URL=https://dev-h6x5kvkxhspo860c.us.auth0.com/.well-known/openid-configuration

# Audiences
LEDGER_API_AUTH_AUDIENCE=https://canton-test.scopex.app
VALIDATOR_AUTH_AUDIENCE=https://canton-test.scopex.app

# M2M credentials
VALIDATOR_AUTH_CLIENT_ID=<m2m-client-id>
VALIDATOR_AUTH_CLIENT_SECRET=<m2m-client-secret>
LEDGER_API_ADMIN_USER=<m2m-client-id>@clients

# Wallet admin user
WALLET_ADMIN_USER=auth0|69ab321f2c22fd609fadc812

# UI app client IDs
WALLET_UI_CLIENT_ID=<wallet-spa-client-id>
ANS_UI_CLIENT_ID=<ans-spa-client-id>
```

---

## Token Types

### 1. M2M Token (Client Credentials)

Used for admin operations on the participant node (party allocation, user creation).

```bash
POST https://dev-h6x5kvkxhspo860c.us.auth0.com/oauth/token
{
  "grant_type": "client_credentials",
  "client_id": "<m2m-client-id>",
  "client_secret": "<m2m-client-secret>",
  "audience": "https://canton-test.scopex.app"
}
```

Token `sub` claim = `<client-id>@clients`

### 2. User Token (Password Grant)

Used for wallet operations (transfers, onboarding, pre-approvals).

```bash
POST https://dev-h6x5kvkxhspo860c.us.auth0.com/oauth/token
{
  "grant_type": "password",
  "client_id": "<m2m-client-id>",
  "client_secret": "<m2m-client-secret>",
  "audience": "https://canton-test.scopex.app",
  "username": "user@example.com",
  "password": "<password>",
  "scope": "openid profile email",
  "connection": "Username-Password-Authentication"
}
```

Token `sub` claim = `auth0|<user-id>` — this becomes the Canton party hint.

### 3. Management API Token

Used to create Auth0 users programmatically.

```bash
POST https://dev-h6x5kvkxhspo860c.us.auth0.com/oauth/token
{
  "grant_type": "client_credentials",
  "client_id": "<m2m-client-id>",
  "client_secret": "<m2m-client-secret>",
  "audience": "https://dev-h6x5kvkxhspo860c.us.auth0.com/api/v2/"
}
```

### 4. Unsafe JWT (Local Dev Only)

Used for local development with `AUTH_MODE=unsafe`.

```bash
HS256 JWT signed with secret "unsafe"
Claims: { iat, exp, aud, sub: <userID> }
```

⚠️ Never use in production. The validator must be started without the `-a` flag for this to work.

---

## Code Structure

```bash
cantonvalidator/
└── auth.go
    ├── GenerateToken()           # entry point — switches unsafe/auth0 M2M
    ├── GetUserToken()            # entry point — user password grant
    ├── generateUnsafeJWT()       # local dev only
    ├── getAuth0TokenCached()     # M2M token with cache
    ├── fetchAuth0Token()         # raw M2M token fetch
    ├── fetchUserToken()          # raw user token fetch
    ├── fetchAuth0ManagementToken() # management API token
    ├── createAuth0User()         # create user via management API
    └── InvalidateCache()         # force fresh M2M token
```

---

## Token Usage by Operation

| Operation | Function | Token Type |
| --- | --- | --- |
| Allocate party | `CreateParty()` | M2M (`GenerateToken`) |
| Create Canton user | `CreateUser()` | M2M (`GenerateToken`) |
| Onboard wallet | `OnboardWallet()` | User (`GetUserToken`) |
| Transfer CC | `Transfer()` | User (`GetUserToken`) |
| Pre-approve transfers | `PreApproveTransfers()` | User (`GetUserToken`) |
| Create Auth0 user | `createAuth0User()` | Management API |
| Query active contracts | `GetActiveContracts()` | M2M (`GenerateToken`) |
| List users | `GetAllWallets()` | M2M (`GenerateToken`) |

---

## Validator Node Startup

The validator must be started with the `-a` flag to enable Auth0:

```bash
export IMAGE_TAG=0.5.8
./start.sh \
  -s "https://sv.sv-1.test.global.canton.network.sync.global" \
  -o "<onboarding-secret>" \
  -p "scopex-validator-1" \
  -m "1" \
  -a \
  -w
```

Without `-a`, the script automatically includes `compose-disable-auth.yaml` which overrides all auth config with `hs-256-unsafe`.

---

## Canton Party ID and Auth0

When a wallet is onboarded via `/register`, Canton derives the party ID from the Auth0 token's `sub` claim:

```bash
Auth0 sub:  auth0|69ab55e66641fc792a480a60
Canton party: auth0_007c69ab55e66641fc792a480a60::12205f40...
```

This means:

- The party ID in the wallet UI will be Auth0-based, not the `walletName` passed to `CreateParty`
- Always use the party ID returned from `OnboardWallet()` — not `CreateParty()` — as the canonical party ID
- Store this in `wallets/wallets.json` via the `db` package

---

## Wallet Store

Credentials and party IDs are stored in `wallets/wallets.json`:

```json
{
  "wallets": [
    {
      "name": "my-wallet",
      "email": "my-wallet@scopex.money",
      "password": "<password>",
      "auth0_user_id": "auth0|xxxxxxxxx",
      "canton_user_id": "my-wallet-user",
      "party_id": "auth0_007cxxxxxxxxx::12205f40..."
    }
  ]
}
```
