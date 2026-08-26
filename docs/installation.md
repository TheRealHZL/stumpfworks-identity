# Guided Installation

The interactive installer supports the roles that can be reproduced safely:

```bash
make build
sudo ./scripts/install-interactive.sh
```

For a Windows-to-Linux release bundle, run `make build-linux-amd64` in a shell with Go available and use the binaries from `dist/linux-amd64`.

Or select a role directly:

```bash
sudo ./scripts/install-interactive.sh login01
sudo ./scripts/install-interactive.sh client
sudo ./scripts/install-interactive.sh dc01-monitoring
```

The installer never downloads packages, creates AD accounts, changes Samba AD, generates a CA or disables TLS validation. It validates required commands and artifacts, prints a summary, asks before installation and creates a new timestamped rollback under `/root`.

## LOGIN01

Prepare these files before starting:

- Linux `sw-badge-server` binary
- LOGIN01 TLS certificate and matching private key
- PKINIT CA certificate and matching private key
- LDAP service-account UPN and password
- AD base DN, administrative group DN, DNS domain and realm
- DC LDAPS certificate SHA-256 pin when using the legacy Samba certificate

The installer verifies certificate/key pairs and expiry, creates the restricted `swbadge` account and paths, stores secrets in separate restricted files, writes the production configuration, installs the hardened service and maintenance timer, then asks separately whether it may start the service.

## Linux client

Prepare:

- Linux `sw-badge-pam-helper` binary
- Linux `sw-badge-client-status` binary and a token provisioned on LOGIN01 for the exact client ID
- trusted Homelab CA certificate
- working AD membership through Winbind/PAM
- LightDM, GTK3 LightDM bindings, `zbarcam`, Kerberos tools including Debian's `krb5-pkinit` package, and the camera

The installer asks for LOGIN01 URL, unique client ID, its provisioned client-status token, realm, camera resolution and whether FILE01 mounts should be installed. It installs a root-controlled client configuration, root-only status token, native greeter, status collector and timer, wrappers, camera helper, PAM helper, XGreeter entry and LightDM configuration. It backs up `/etc/pam.d/lightdm` and inserts exactly one isolated `pam_exec` rule before `common-auth`, preserving the password fallback. Restarting LightDM requires a separate confirmation because it ends the graphical session. The former role name `wyse01` remains accepted as a compatibility alias, but new installations should use `client`.

The v1.1 greeter uses NetworkManager to show the real LAN/WLAN and LOGIN01 state before login. On systems with a Wi-Fi adapter it offers a network chooser at the LightDM screen. Wi-Fi secrets are passed to NetworkManager through standard input, never command-line arguments or SWBA logs, and are stored by NetworkManager. The installer requires `nmcli` and installs a narrowly scoped polkit rule for the dedicated `lightdm` account. Test wired networking, a saved Wi-Fi profile, a new protected Wi-Fi network and the password fallback before production rollout.

The installer also asks for the camera scan resolution. Keep `640x480` for the confirmed WYSE USB camera. The Broadcom FaceTime HD camera on MOBILE01 exposes `1280x720` and therefore uses that native mode.

Reusable Samba/Winbind and Kerberos member templates are stored in `deploy/client`. Keep the `rid` ranges identical on every workstation so an AD account receives the same Unix UID everywhere. Joining the domain remains an explicit administrator step: never place domain credentials in an installer argument, shell history or repository. Run the interactive `net ads join -U <admin>` command locally on the new client and enter the password only at its protected prompt.

The optional `deploy/client/pam_mount.conf.xml` template mounts the user's FILE01 home share at `~/Netzlaufwerk` and the common `public` share at `~/Öffentlich`. Both mounts require the PKINIT-created user TGT through `sec=krb5,cruid=%(USERUID)`; they contain no stored SMB password.

## DC01

`dc01-monitoring` installs only the daily KDC/PKINIT certificate check after confirming certificate paths and a current backup. It does not install PKINIT, replace Kerberos configuration, restart Samba or modify AD.

PKINIT CA creation and initial DC01 KDC configuration remain a controlled manual procedure. They require a verified current Proxmox backup, inspection of the live Samba configuration, certificate extension validation, a dedicated rollback and a real post-change PKINIT login test. Automating those decisions blindly would make the installer less safe, not easier.

## Verification

After installation, follow the end-to-end acceptance test in [Operations](operations.md). A successful service start alone is not sufficient: confirm badge/PIN login, a fresh TGT, CIFS ticket, both mounts and the normal AD password fallback.
