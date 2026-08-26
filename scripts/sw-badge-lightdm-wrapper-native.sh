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

/usr/local/bin/sw-badge-native-greeter &
badge_pid=$!

# LightDM's X session has no regular window manager, so explicitly size,
# raise and focus the native top-level window after GTK creates it.
(
    i=0
    while [ "$i" -lt 50 ]; do
        window=$(xdotool search --onlyvisible --class sw-badge-native-greeter 2>/dev/null | tail -n 1 || true)
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

wait "$gtk_pid"
status=$?
gtk_pid=""
exit "$status"
