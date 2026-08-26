#!/bin/sh
set -eu

if [ "$#" -lt 2 ]; then
    echo "usage: $0 <warning-seconds> <certificate> [certificate ...]" >&2
    exit 2
fi

warning_seconds=$1
shift

case "$warning_seconds" in
    *[!0-9]*|'') echo "warning period must be a positive number of seconds" >&2; exit 2 ;;
esac

failed=0
for certificate in "$@"; do
    if [ ! -r "$certificate" ]; then
        echo "CRITICAL: certificate is not readable: $certificate" >&2
        failed=1
        continue
    fi
    subject=$(openssl x509 -in "$certificate" -noout -subject 2>/dev/null) || {
        echo "CRITICAL: invalid certificate: $certificate" >&2
        failed=1
        continue
    }
    expiry=$(openssl x509 -in "$certificate" -noout -enddate | cut -d= -f2-)
    if openssl x509 -in "$certificate" -noout -checkend "$warning_seconds" >/dev/null; then
        echo "OK: $certificate; $subject; expires=$expiry"
    else
        echo "CRITICAL: certificate expires within warning period: $certificate; $subject; expires=$expiry" >&2
        failed=1
    fi
done

exit "$failed"
