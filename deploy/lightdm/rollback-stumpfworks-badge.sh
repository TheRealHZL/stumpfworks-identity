#!/bin/sh
set -eu

rm -f /etc/lightdm/lightdm.conf.d/60-stumpfworks-badge.conf
systemctl restart lightdm
