#!/bin/sh
set -eu

backup=/root/swbadge-pkinit-rollback
test -f "$backup/etc-krb5.conf"
test -f "$backup/private-krb5.conf"

install -o root -g root -m 0644 "$backup/etc-krb5.conf" /etc/krb5.conf
install -o root -g root -m 0644 "$backup/private-krb5.conf" /var/lib/samba/private/krb5.conf
systemctl restart samba-ad-dc
