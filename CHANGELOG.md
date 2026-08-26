# Changelog

All notable changes to StumpfWorks Identity are documented here.

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
