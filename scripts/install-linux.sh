#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then echo "Run as root" >&2; exit 1; fi
id swbadge >/dev/null 2>&1 || useradd --system --home /var/lib/stumpfworks-badge --shell /usr/sbin/nologin swbadge
install -d -o swbadge -g swbadge /opt/stumpfworks-badge /var/lib/stumpfworks-badge /var/log/stumpfworks-badge
install -d -m 0750 /etc/stumpfworks-badge
install -m 0755 bin/sw-badge-server /opt/stumpfworks-badge/sw-badge-server
install -m 0640 configs/config.example.yaml /etc/stumpfworks-badge/config.yaml
install -m 0644 deploy/systemd/sw-badge-server.service /etc/systemd/system/sw-badge-server.service
chown root:swbadge /etc/stumpfworks-badge/config.yaml
systemctl daemon-reload
systemctl enable sw-badge-server.service
echo "Installed. Review /etc/stumpfworks-badge/config.yaml, then start the service."
