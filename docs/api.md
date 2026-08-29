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

User, badge, directory and QR operations require a signed administrator session and CSRF for writes. `/self-service` authenticates a normal user over LDAPS, permits only that identity's PIN update and shows only active badges assigned to that local identity. The bounded badge view contains the badge ID, optional description, issue date and last-use date; it never contains token hashes or another user's badges. Its session cannot act as an administrator and expires after 15 minutes.

`POST /self-service/badges/{id}/revoke` reports an active badge as lost. The database update requires the badge ID, authenticated local user ID and active state to match in one transaction. Foreign, unknown and already-disabled badges share the same response, and a successful action creates a secret-free `badge_self_service_revoked` audit event.

The authenticated self-service page also shows at most the latest 20 badge-authentication events associated with that signed-in username. Each row is limited to timestamp, badge code, client ID and result. IP addresses, internal audit details, tokens and other users' events are never included in this view.

`POST /self-service/badges/activate` accepts a complete one-time replacement payload. Activation succeeds only when its token matches, the badge is explicitly pending activation, and its owner matches the signed self-service identity. Revoked or ordinary disabled badges cannot be reactivated through this route.

`POST /self-service/sessions/logout-others` revokes all other server-tracked self-service sessions for the signed identity while preserving the current session. Session records contain a random identifier, username and bounded timestamps, but no AD password or session-cookie value.

The protected badge administration page exposes the same replacement operation at `POST /badges/{id}/replace` with CSRF protection and redirects to the one-time payload display. The previous badge is revoked and the replacement remains pending until owner activation.
