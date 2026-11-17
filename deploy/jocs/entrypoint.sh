#!/bin/sh
# Container startup for Gregal Jocs: sets up the administrator (if
# JOCS_ADMIN is set) and starts the server. Data lives in /data.
set -eu
CFG="$HOME/.config/gregal-jocs/config.yaml"
mkdir -p "$HOME/accounts" "$HOME/work"

if [ -n "${JOCS_ADMIN:-}" ]; then
  [ -f "$CFG" ] || gregal-jocs init --config="$CFG" >/dev/null
  if ! grep -q '^users:' "$CFG"; then
    cat >>"$CFG" <<YAML

users:
  $JOCS_ADMIN:
    admin: true
    roots:
      - $HOME/work
    home: $HOME/work
YAML
    chmod 0600 "$CFG"
  fi
  # The password in .env wins on every start: to change it, edit it and
  # restart (deploy.sh --restart), or change it from the web UI.
  if [ -n "${JOCS_ADMIN_PASSWORD:-}" ]; then
    printf '%s\n' "$JOCS_ADMIN_PASSWORD" | gregal-jocs --config "$CFG" --passwd "$JOCS_ADMIN" >/dev/null 2>&1 ||
      echo "entrypoint: could not set the password of $JOCS_ADMIN" >&2
  fi
fi
exec gregal-jocs --serve --addr 0.0.0.0 --port 8197
