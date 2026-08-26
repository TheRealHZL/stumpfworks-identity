# Contributing

## Branch model

- `main` is release-only and must remain deployable.
- `development` collects tested changes for the next release.
- Create `feature/<short-name>` or `fix/<short-name>` from `development`.
- Open pull requests into `development`; merge `development` into `main` only for a verified release.

Do not commit directly to `main`. Prefer small commits that describe one coherent change.

## Required checks

```bash
go test ./...
python3 -m py_compile cmd/native-greeter/sw-badge-native-greeter.py
python3 -m py_compile scripts/swbadge-sqlite-backup.py
sh -n scripts/*.sh
git diff --check
```

Changes to LightDM, PAM, Kerberos, PKINIT or Samba AD also require an end-to-end test on a non-production client before release.

## Privacy and secrets

Never commit real infrastructure details or authentication material, including:

- passwords, PINs, badge payloads, grants or API tokens;
- private keys, certificate bundles or Kerberos caches;
- production IP addresses, domains, usernames or certificate fingerprints;
- databases, audit logs, backups, screenshots containing identifiers or local handover notes.

Use `example.test`, RFC 5737 documentation addresses and fictional users in committed examples. Keep deployment-specific values in ignored local files.
