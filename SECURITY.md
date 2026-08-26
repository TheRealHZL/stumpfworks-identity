# Security Policy

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability, leaked credential or deployment-specific detail. Use GitHub's private vulnerability reporting feature for this repository.

Include the affected component, impact, reproduction steps and any proposed mitigation. Do not include live credentials, badge payloads, private keys or personal data in the report.

## Supported versions

Security fixes target the latest tagged release and the current `development` branch. Older releases may require an upgrade before a fix can be applied.

## Deployment responsibility

SWBA integrates with PAM, Kerberos, PKINIT and Active Directory. Test changes in an isolated environment, retain password fallback and a rollback path, and back up production identity systems before modifying them.
