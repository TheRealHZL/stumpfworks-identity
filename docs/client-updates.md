# Signed client updates

The first update increments are verification and isolated staging only. `sw-badge-client-updater` cannot install files and exits unless `--dry-run` is supplied.

An update ZIP contains exactly:

- `manifest.json`
- `manifest.sig`, a base64 Ed25519 signature over the exact manifest bytes
- one `payload/<component>` entry for every manifest file

The schema records release version, channel (`development` or `stable`), target OS/architecture, minimum client version, creation time, and the exact size and SHA-256 digest of every component. Component names come from a fixed allowlist; paths, commands and installation destinations cannot be supplied by a package.

Verification rejects unknown manifest fields, trailing JSON, duplicate or unlisted ZIP entries, unsafe paths, unsupported components, malformed versions, wrong architecture, excessive sizes, invalid hashes and invalid signatures. The release public key is provided separately as a base64 Ed25519 key. A private release key must never be installed on LOGIN01 or a client and must never be committed.

Example dry run:

```bash
sw-badge-client-updater \
  --dry-run \
  --package stumpfworks-client-1.2.1-linux-amd64.zip \
  --public-key /etc/stumpfworks-badge/update-release.pub
```

An optional `--stage-dir` extracts verified components into a new mode-restricted directory and writes `stage-plan.json`. Existing directories are never overwritten. The plan contains only fixed component-to-target mappings compiled into the updater, expected modes and whether a future installation would require a LightDM restart. Payload hashes are checked again while staging to close the verification-to-extraction gap. Staging never writes to an installation target.

Rollback preparation uses the fixed stage plan to copy only known existing target files into a new mode `0700` directory. Each saved file is mode `0600`, hashed, size-limited and recorded in `rollback.json`; targets that did not previously exist are recorded explicitly. Symlinks, special files, duplicate components, changed target mappings and existing rollback directories are rejected.

The internal restore implementation validates the complete manifest and every backup before changing any target. Existing files are restored through same-directory atomic replacements; files recorded as previously absent are removed. A restore can be rerun safely after an injected mid-restore failure. Restore is not exposed through the CLI yet, and package installation remains disabled until installation failure tests invoke this restore automatically.

The internal installation transaction now revalidates the complete staged payload, creates the rollback snapshot, prepares replacement files and atomically replaces fixed targets. Any preparation or application error invokes rollback automatically. A successful transaction requires a non-optional post-install health callback; a failed health result restores the old files after proving that the new files were visible to the check. Failure-injection tests also prove that a tampered stage is rejected before a rollback directory or target change is created. Installation remains absent from the public CLI while the health gates are exercised beyond their simulated command runner.

Concrete orchestration now requires an explicit maintenance-window flag for greeter components. Non-greeter updates reload systemd only when unit files changed, submit an immediate client-status report when relevant, require the status timer to remain active and always require LightDM to remain active. Command failures propagate into the mandatory health gate and therefore trigger automatic rollback. These commands are covered by a simulated runner; the installation path remains unavailable from the CLI.
