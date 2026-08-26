#!/bin/sh
set -eu

gtk_pid=""
badge_pid=""

cleanup() {
    if [ -n "$badge_pid" ]; then
        kill "$badge_pid" 2>/dev/null || true
        wait "$badge_pid" 2>/dev/null || true
    fi
    if [ -n "$gtk_pid" ]; then
        kill "$gtk_pid" 2>/dev/null || true
        wait "$gtk_pid" 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

"$@" &
gtk_pid=$!

export SWBADGE_AUTHORIZED_COMMAND=/usr/local/bin/sw-badge-lightdm-handoff
/usr/local/bin/sw-badge-greeter-start &
badge_pid=$!

wait "$gtk_pid"
status=$?
gtk_pid=""
exit "$status"
