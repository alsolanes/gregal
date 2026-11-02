#!/bin/sh
# Consistent, private snapshot of the standard systemd installation.
set -eu
[ "$(id -u)" = 0 ] || { echo 'Run as root (sudo).' >&2; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo 'systemctl is required.' >&2; exit 1; }
state=$(systemctl show --property=ActiveState --value gregal) || { echo 'Cannot check Gregal service state.' >&2; exit 1; }
case "$state" in
  inactive|failed) ;;
  *) echo 'Stop Gregal first: sudo systemctl stop gregal' >&2; exit 1 ;;
esac
[ -d /var/lib/gregal ] || { echo '/var/lib/gregal does not exist.' >&2; exit 1; }
[ -f /etc/gregal/env ] || { echo '/etc/gregal/env does not exist.' >&2; exit 1; }
backup_dir=${1:-/var/backups/gregal}
umask 077
mkdir -p -- "$backup_dir"
backup_dir=$(cd "$backup_dir" && pwd -P)
case "$backup_dir/" in
  /var/lib/gregal/*|/etc/gregal/*) echo 'Backup destination must be outside the data directories.' >&2; exit 1 ;;
esac
file=$(mktemp "$backup_dir/gregal-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX.tar.gz")
trap 'rm -f -- "$file"' EXIT
trap 'exit 1' HUP INT TERM
# Relative paths avoid restoring an archive over live absolute paths by accident.
tar -C / -czf "$file" var/lib/gregal etc/gregal/env
tar -tzf "$file" >/dev/null
trap - EXIT HUP INT TERM
printf 'Backup created: %s\n' "$file"
