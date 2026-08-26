# API

Machine endpoints use `/api/v1`; failures use `{"valid":false,"reason":"..."}`. Production requests require verified HTTPS.

## Health

`GET /api/v1/health` returns `{"status":"ok"}` without authentication.

## Badge authentication

```http
POST /api/v1/auth/badge
Content-Type: application/json

{"badge_id":"SW-0001","token":"<TOKEN>","pin":"<PIN>","client_id":"wyse01-greeter"}
```

Success returns mapped identity, badge ID and a short-lived grant. Denials include `badge_unknown`, `badge_disabled`, `invalid_token`, `pin_required`, `invalid_pin` and `rate_limited`.

## PKINIT exchange

```http
POST /api/v1/auth/pkinit
Content-Type: application/json

{"login_grant":"<ONE_TIME_GRANT>","client_id":"wyse01-greeter"}
```

The grant works only once, before expiry and for its client. The response contains short-lived certificate material and must never be logged.

## Administration and self-service

User, badge, directory and QR operations require a signed administrator session and CSRF for writes. `/self-service` authenticates a normal user over LDAPS and permits only that identity's PIN update. Its session cannot act as an administrator and expires after 15 minutes.
