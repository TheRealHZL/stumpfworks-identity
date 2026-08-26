#!/bin/sh
set -eu

greeter_pid=""
cleanup() {
    if [ -n "$greeter_pid" ]; then
        kill "$greeter_pid" 2>/dev/null || true
        wait "$greeter_pid" 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

export SWBADGE_CAMERA_PREVIEW=0
if [ -n "${SWBADGE_AUTHORIZED_COMMAND:-}" ]; then
    /opt/stumpfworks-badge/sw-badge-greeter \
        --server https://login01.example.test:8080 \
        --client-id client01-greeter \
        --ca-file /etc/stumpfworks-badge/stumpfworks-homelab-ca.crt \
        --camera-helper /usr/local/bin/sw-badge-camera-linux \
        --authorized-command "$SWBADGE_AUTHORIZED_COMMAND" \
        --listen 127.0.0.1:18081 &
else
    /opt/stumpfworks-badge/sw-badge-greeter \
        --server https://login01.example.test:8080 \
        --client-id client01-greeter \
        --ca-file /etc/stumpfworks-badge/stumpfworks-homelab-ca.crt \
        --camera-helper /usr/local/bin/sw-badge-camera-linux \
        --listen 127.0.0.1:18081 &
fi
greeter_pid=$!

sleep 1
profile_dir="${XDG_CACHE_HOME:-$HOME/.cache}/swbadge-greeter-firefox"
mkdir -p "$profile_dir"

# LightDM's greeter X session has no regular window manager. Give the kiosk
# explicit keyboard focus once Firefox creates its top-level window.
(
    i=0
    while [ "$i" -lt 50 ]; do
        window=$(xdotool search --onlyvisible --class firefox-esr 2>/dev/null | tail -n 1 || true)
        if [ -n "$window" ]; then
            geometry=$(xdotool getdisplaygeometry)
            width=${geometry% *}
            height=${geometry#* }
            xdotool windowmove "$window" 0 0
            xdotool windowsize "$window" "$width" "$height"
            xdotool windowraise "$window"
            xdotool windowfocus --sync "$window"
            exit 0
        fi
        i=$((i + 1))
        sleep 0.2
    done
) &
firefox-esr --no-remote --new-instance --profile "$profile_dir" --kiosk http://127.0.0.1:18081/
