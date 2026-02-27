# Steps to create wallet

## 1. Allocate party

```bash
grpcurl -plaintext \
  -d '{"party_id_hint": "<PARTY_NAME>"}' \
  localhost:5001 \
  com.daml.ledger.api.v2.admin.PartyManagementService/AllocateParty
```

## 2. Create user (replace `<PARTY_ID>` with output from step 1)

```bash
grpcurl -plaintext \
  -d '{
    "user": {
      "id": "<PARTY_NAME>-user",
      "primary_party": "<PARTY_ID>"
    },
    "rights": [
      {"can_act_as": {"party": "<PARTY_ID>"}},
      {"can_read_as": {"party": "<PARTY_ID>"}}
    ]
  }' \
  localhost:5001 \
  com.daml.ledger.api.v2.admin.UserManagementService/CreateUser
```

## 3. Onboard via validator API

```bash
TOKEN=$(python3 -c "import jwt,time; print(jwt.encode({'iat':int(time.time()),'aud':'https://validator.example.com','sub':'test-wallet-user'},'unsafe',algorithm='HS256'))")

python3 -c "
import urllib.request, json, urllib.error
token = '$TOKEN'
req = urllib.request.Request('http://localhost:8080/api/validator/v0/register', method='POST', data=json.dumps({}).encode(), headers={'Content-Type':'application/json','Authorization':'Bearer '+token})
try:
    r = urllib.request.urlopen(req, timeout=10)
    print(r.read().decode())
except urllib.error.HTTPError as e:
    print(e.code, e.read().decode())
"
```

## We get the following data

| What we set               | Value                         |
|-------------------|-------------------------------|
| Party hint    | <PARTY_NAME>                      |
| Party ID | <PARTY_ID>      |
| User ID (login username)    | <PARTY_NAME>-user                  |
