#!/bin/sh
set -eu

config=/etc/stumpfworks-badge/client.conf
if [ ! -r "$config" ]; then
    echo "SWBA client configuration is missing: $config" >&2
    exit 1
fi

# This file is installed root-owned; quote all values in client.conf.
. "$config"

exec /usr/local/libexec/sw-badge-pam-helper \
    -server "$SWBADGE_SERVER" \
    -client-id "$SWBADGE_CLIENT_ID" \
    -ca-file "$SWBADGE_CA_FILE" \
    -grant-file "$SWBADGE_GRANT_FILE" \
    -realm "$SWBADGE_REALM"
