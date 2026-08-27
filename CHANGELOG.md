# Changelog

All notable changes to StumpfWorks Identity are documented here.

## [Unreleased]

### Added

- Server-side client registration with one-time random credentials stored only as hashes.
- Authenticated, data-minimized client health reporting and a protected client-list API.
- A hardened one-shot Linux status collector with a periodic systemd timer.
- A protected administrative client-health overview with explicit stale-report detection.
- Local client-token rotation and reversible status-credential disabling without exposing secrets to the web interface.
- Client/server semantic-version comparison with an update-availability indicator in the protected status page.
- Root-only, privacy-limited client diagnostic archives and an automatic post-installation system check.
- Verification-only client updater with signed manifests, fixed components, architecture checks and package-integrity limits.
- Isolated atomic staging with fixed installation targets, a machine-readable plan and a second payload-integrity check.
- Atomic rollback snapshot preparation with source hashes, missing-file records and symlink rejection.
- Prevalidated, resumable rollback restoration with atomic file replacement and failure-injection tests.
- Internal atomic installation transaction with automatic rollback and tampered-stage rejection, still unavailable from the CLI.
- Mandatory post-install health gate with automatic restoration after an injected unhealthy result.
- Component-aware systemd health orchestration and an explicit maintenance-window requirement for greeter updates.
- Guarded client-updater installation mode with mandatory fresh staging and rollback paths.
- Post-rollback systemd recovery checks and explicit LightDM restart handling for approved greeter maintenance.
- Create-only Ed25519 development key generation and signed update packaging with fixed component validation.

## [1.1.0] - 2026-08-26

### Added

- Pre-login LAN/WLAN state and a NetworkManager-backed Wi-Fi chooser.
- Generic guided Linux-client installation with role-specific validation and rollback files.
- Root-controlled shared client configuration for the greeter and PAM helper.
- Persistent, password-protected VNC service template with a safe loopback default.
- Polkit policy restricted to the dedicated LightDM greeter account.
- Public repository hygiene, CI, contribution guidance and security reporting policy.
- An anonymized native-greeter screenshot and expanded project documentation.

### Changed

- Renamed the project from StumpfWorks Badge Login to StumpfWorks Identity.
- Improved native-greeter buttons, pointer visibility and device-name handling.
- Replaced deployment-specific examples with reserved `example.test` values.
- Moved development to a dedicated `development` branch while keeping `main` release-only.
- Updated the Go module path for the new repository.

### Fixed

- Ensured the greeter and PAM helper use one TLS-verified hostname and CA configuration.
- Ensured LightDM can read the trusted CA without exposing root-only client files.
- Preserved Kerberos-backed FILE01 mounts after badge authentication.
- Normalized Windows line endings during guided Linux-client installation.

### Compatibility

Existing `SWBADGE_*` environment variables, API paths, badge payloads and installed file names remain supported. No badge reissue or database migration is required.

## [1.0.0] - 2026-08-23

- Initial stable release with native LightDM badge/PIN login, AD password fallback, one-time grants, PKINIT, Kerberos TGT creation, CIFS mounts, administration, PIN self-service, auditing, backups and certificate monitoring.
