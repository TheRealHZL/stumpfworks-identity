#!/bin/sh
set -eu

[ "$(id -u)" -eq 0 ] || { echo "Run as root." >&2; exit 1; }
command -v x11vnc >/dev/null 2>&1 || { echo "x11vnc is not installed." >&2; exit 1; }

install -d -o root -g root -m 0700 /etc/stumpfworks-badge
password_tmp=$(mktemp /etc/stumpfworks-badge/.vnc.passwd.XXXXXX)
trap 'rm -f "$password_tmp"' EXIT INT TERM

echo "Enter a strong VNC password with exactly 8 characters."
echo "Classic VNC authentication ignores or rejects characters beyond byte 8."
echo "The password is entered at the protected prompt and is never printed or logged."
x11vnc -storepasswd "$password_tmp"
install -o root -g root -m 0600 "$password_tmp" /etc/stumpfworks-badge/vnc.passwd
rm -f "$password_tmp"
trap - EXIT INT TERM

systemctl enable --now swbadge-vnc.service
systemctl --no-pager --full status swbadge-vnc.service
