#!/bin/sh
set -eu

camera_device="${SWBADGE_CAMERA_DEVICE:-/dev/video0}"
camera_preview="${SWBADGE_CAMERA_PREVIEW:-auto}"
camera_resolution="${SWBADGE_CAMERA_RESOLUTION:-640x480}"

case "$camera_resolution" in
    *[!0-9x]*|*x*x*|x*|*x) echo "invalid camera resolution: $camera_resolution" >&2; exit 1 ;;
esac
camera_width=${camera_resolution%x*}
camera_height=${camera_resolution#*x}

if ! command -v zbarcam >/dev/null 2>&1; then
    echo "zbarcam is missing; install the Debian package zbar-tools" >&2
    exit 1
fi

if [ ! -r "$camera_device" ]; then
    echo "camera device is not readable: $camera_device" >&2
    exit 1
fi

# USB webcams can retain an impractically large format from a previous app.
# Use a modest, QR-friendly mode consistently on thin clients.
if command -v v4l2-ctl >/dev/null 2>&1; then
    v4l2-ctl --device "$camera_device" \
        --set-fmt-video=width="$camera_width",height="$camera_height",pixelformat=YUYV \
        --set-parm=30 >/dev/null
fi

if [ -n "${DISPLAY:-}" ] && [ "$camera_preview" != "0" ]; then
    echo "Opening camera preview; hold one StumpfWorks badge in view..." >&2
    payload="$(zbarcam --quiet --raw --oneshot --prescale="$camera_resolution" --set disable --set qrcode.enable "$camera_device")"
else
    echo "No graphical display detected; scanning headless..." >&2
    payload="$(zbarcam --quiet --raw --oneshot --nodisplay --prescale="$camera_resolution" --set disable --set qrcode.enable "$camera_device")"
fi

case "$payload" in
    SWBADGE:1:*)
        printf '%s\n' "$payload"
        ;;
    *)
        echo "camera detected a QR code, but it is not a StumpfWorks badge" >&2
        exit 1
        ;;
esac
