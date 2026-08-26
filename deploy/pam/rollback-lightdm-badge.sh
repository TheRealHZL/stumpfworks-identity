#!/bin/sh
set -eu

backup=/root/swbadge-lightdm-badge-pam-rollback/lightdm.before
test -f "$backup"
install -o root -g root -m 0644 "$backup" /etc/pam.d/lightdm
rm -f /run/swbadge/login-grant
systemctl restart lightdm
