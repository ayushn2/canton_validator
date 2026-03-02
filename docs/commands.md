# Commands

## Start Validator Node

```bash
./start.sh -s "https://sv.sv-1.test.global.canton.network.sync.global" -o "<TESTNET_SECRET>" -p "scopex-validator-1" -m "1" -w
```

## Pre-Approve transfers

```bash
TOKEN=$(python3 -c "import jwt,time; print(jwt.encode({'iat':int(time.time()),'aud':'https://validator.example.com','sub':'<USER_ID>'},'unsafe',algorithm='HS256'))")

python3 -c "
import urllib.request, json, urllib.error
token = '$TOKEN'
req = urllib.request.Request(
    'http://localhost:8080/api/validator/v0/wallet/transfer-preapproval',
    method='POST',
    data=json.dumps({}).encode(),
    headers={
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + token
    }
)
try:
    r = urllib.request.urlopen(req, timeout=10)
    print('Pre-approval created:', r.read().decode())
except urllib.error.HTTPError as e:
    print('Error:', e.code, e.read().decode())
"
```

## Enter canton console

```bash
docker exec -it splice-validator-participant-1 \
  /app/bin/canton --config /tmp/remote.conf
```

## Find Traffic rate

```bash
participant.traffic_control.traffic_state(participant.synchronizers.list_connected().head.synchronizerId)
```
