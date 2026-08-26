#!/bin/sh
set -eu

username=${1:-}
case "$username" in
    ""|*[!A-Za-z0-9._-]* ) exit 2 ;;
esac

IFS= read -r grant || exit 2
case "$grant" in
    ""|*[!A-Za-z0-9_-]* ) exit 2 ;;
esac
if [ "${#grant}" -lt 32 ] || [ "${#grant}" -gt 128 ]; then
    exit 2
fi

umask 077
grant_tmp="/run/swbadge/login-grant.$$"
printf '%s\n%s\n' "$username" "$grant" > "$grant_tmp"
mv "$grant_tmp" /run/swbadge/login-grant

# The LightDM greeter has no regular EWMH window manager. Hide only the
# Firefox kiosk window directly through X11; keep the process for cleanup.
firefox_window=$(xdotool search --onlyvisible --class 'firefox-esr' 2>/dev/null | tail -n 1 || true)
if [ -n "$firefox_window" ]; then
    xdotool windowunmap "$firefox_window"
fi

# Bring the original GTK greeter forward, enter the validated AD name, and
# continue to its normal PAM password prompt.
i=0
while [ "$i" -lt 25 ]; do
    window=$(xdotool search --onlyvisible --class 'lightdm-gtk-greeter' 2>/dev/null | tail -n 1 || true)
    if [ -n "$window" ]; then
        xdotool windowraise "$window"
        xdotool windowfocus --sync "$window"
        xdotool key --clearmodifiers ctrl+a
        xdotool type --clearmodifiers --delay 35 "$username"
        xdotool key --clearmodifiers Return
        exit 0
    fi
    i=$((i + 1))
    sleep 0.2
done

exit 1
