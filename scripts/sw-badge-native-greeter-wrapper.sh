#!/bin/sh
set -eu

config=/etc/stumpfworks-badge/client.conf
if [ ! -r "$config" ]; then
    echo "SWBA client configuration is missing: $config" >&2
    exit 1
fi

# This file is installed root-owned and is not writable by the greeter.
. "$config"
export SWBADGE_SERVER SWBADGE_CLIENT_ID SWBADGE_CA_FILE
export SWBADGE_CAMERA_HELPER SWBADGE_GRANT_FILE
export SWBADGE_CAMERA_RESOLUTION

exec /usr/local/bin/sw-badge-native-greeter
