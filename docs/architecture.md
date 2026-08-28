# Architecture

## Components

`LOGIN01` runs the Go HTTPS server, SQLite identity database, administrator UI, PIN self-service and PKINIT CA. `DC01` is the Samba AD domain controller and KDC. `WYSE01` is an AD member running LightDM, the native GTK3 greeter, camera helper and PAM helper. `FILE01` is an AD member providing Kerberos-protected SMB shares.

The existing Winbind/PAM membership on WYSE01 remains authoritative. SWBA adds one isolated `pam_exec` rule before the normal password stack; it does not replace the domain join or fallback.

## Badge login

1. The camera reads `SWBADGE:1:<BADGE_ID>:<TOKEN>`.
2. The greeter sends badge and PIN to LOGIN01 over verified HTTPS.
3. LOGIN01 validates token hash, badge state, PIN hash, rate limit and client ID.
4. It returns a random grant valid for 30 seconds, one use and one client.
5. The PAM helper consumes and deletes the runtime grant.
6. LOGIN01 exchanges it for a ten-minute user certificate and key.
7. The helper performs PKINIT against DC01 and writes `/tmp/krb5cc_<UID>` as the user with mode `0600`.
8. `pam_mount` uses `sec=krb5,cruid=<UID>` for the personal and public FILE01 shares.

The result is a real user TGT. No synthetic or stored AD password is involved.

## Web authentication

Administrator sessions require the configured AD administrator group. Self-service accepts a valid AD user but can update only that identity's SWBA PIN and read a bounded list of active badges selected by the authenticated local user ID. Signed session tokens carry distinct `admin` and `self-service` audiences and cannot be exchanged. Browser writes use secure, HTTP-only, SameSite strict cookies and CSRF tokens. Self-service expires after 15 minutes.

Lost-badge self-service revocation is constrained again inside the database transaction by both badge ID and authenticated user ID. It never accepts a username or owner ID from form data and never returns whether a foreign badge exists.

## Trust boundaries

- Homelab TLS CA: LOGIN01 HTTPS
- PKINIT CA: short-lived user and KDC certificates
- Samba AD: users, groups, Kerberos and machine trust
- SQLite: mappings, hashes and audit metadata
- Runtime grant file: one-time greeter-to-PAM handoff

Private CA keys remain on LOGIN01. Cleartext PINs and badge tokens are never persisted.

## Availability

Password fallback remains available if badge authentication fails and AD is reachable. LOGIN01 and Samba services start at boot. Daily timers check certificate expiry; LOGIN01 also creates a consistent online SQLite backup.
