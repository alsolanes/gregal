#!/bin/sh
# Deploy and manage Gregal Jocs with Docker, locally or on a server over SSH.
#
#   ./deploy.sh [--host user@server] [--domain jocs.example.com] [--admin NAME]
#        deploy or update (HTTPS with Caddy when --domain is given)
#   ./deploy.sh [--host ...] --logs | --status | --restart | --stop
#   ./deploy.sh [--host ...] --accounts | --invite | --backup
#
# Only Docker (with the compose plugin) is needed on the server. Safe to repeat
# to update: it rebuilds the image and keeps the data (jocs-data volume).
# Full guide: deploy/jocs/GUIDE.md
set -eu
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$HERE/../.." && pwd)
DOMAIN=""; HOST=""; ACTION=up; DIR=gregal-jocs; ADDR=localhost; ADMIN=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain) DOMAIN=${2:?missing domain}; shift 2 ;;
    --host) HOST=${2:?missing user@server}; ADDR=${HOST##*@}; shift 2 ;;
    --dir) DIR=${2:?missing directory}; shift 2 ;;
    --admin) ADMIN=${2:?missing name}; shift 2 ;;
    --logs|--stop|--status|--restart|--accounts|--invite|--backup) ACTION=${1#--}; shift ;;
    -h|--help) sed -n '2,11p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

# Commands run where the deployment lives (local or remote).
run() { if [ -n "$HOST" ]; then ssh "$HOST" "cd $DIR/deploy/jocs && $1"; else (cd "$HERE" && sh -c "$1"); fi; }
# With --domain, Caddy is in the compose file (https profile): every command needs it.
PROFILE=""; [ -n "$DOMAIN" ] && PROFILE="--profile https"
DC="docker compose $PROFILE"
# Call the admin API from inside the container (the master key is in its environment).
api() { run "$DC exec -T jocs sh -c 'wget -qO- $2 --header \"Authorization: Bearer \$GREGAL_API_TOKEN\" http://127.0.0.1:8197/api/server/$1'"; echo; }

case "$ACTION" in
  logs) run "$DC logs -f --tail=100"; exit ;;
  stop) run "$DC down"; exit ;;
  status) run "$DC ps"; exit ;;
  restart) run "$DC restart jocs"; exit ;;
  accounts) api accounts ""; exit ;;
  invite) api invitations "--post-data="; echo "Single-use code, expires in 30 minutes."; exit ;;
  backup)
    run 'mkdir -p backups && f=jocs-$(date +%Y%m%d-%H%M%S).tgz && docker run --rm -v gregal-jocs_jocs-data:/d -v "$PWD/backups":/b alpine tar czf /b/$f -C /d . && echo "Backup: deploy/jocs/backups/$f"'
    exit ;;
esac

VERSION=$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)

if [ -n "$HOST" ]; then
  echo "==> uploading the code to $HOST:~/$DIR"
  # Only what builds gregal-jocs (go.mod, internal, jocs, deploy/jocs).
  (cd "$ROOT" && tar -czf - go.mod go.sum internal jocs deploy/jocs) |
    ssh "$HOST" "mkdir -p $DIR && tar -xzf - -C $DIR"
fi

# .env: master key generated the first time and kept afterwards.
run "[ -f .env ] || {
  umask 077
  KEY=\$(LC_ALL=C tr -dc '23456789abcdefghjkmnpqrstuvwxyz' </dev/urandom | head -c 20)
  printf 'GREGAL_API_TOKEN=%s\nGREGAL_PUBLIC_URL=\n' \"\$KEY\" > .env
  echo '==> new master key created in deploy/jocs/.env'
}"
if [ -n "$DOMAIN" ]; then
  run "sed -i '/^GREGAL_PUBLIC_URL=/d;/^JOCS_BIND=/d;/^JOCS_DOMAIN=/d' .env && printf 'GREGAL_PUBLIC_URL=https://$DOMAIN\nJOCS_BIND=127.0.0.1\nJOCS_DOMAIN=$DOMAIN\n' >> .env"
fi
# Administrator with a password (only if .env has none yet).
if [ -n "$ADMIN" ]; then
  run "grep -q '^JOCS_ADMIN=.' .env || {
    PW=\$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 20)
    sed -i '/^JOCS_ADMIN/d' .env
    printf 'JOCS_ADMIN=$ADMIN\nJOCS_ADMIN_PASSWORD=%s\n' \"\$PW\" >> .env
    echo \"==> administrator $ADMIN created. Password (save it): \$PW\"
  }"
fi

echo "==> building and starting (version $VERSION)"
run "VERSION=$VERSION $DC up -d --build"
echo
echo "Gregal Jocs is running."
[ -n "$DOMAIN" ] && echo "  URL: https://$DOMAIN/jocs/" || echo "  URL: http://$ADDR:8197/jocs/"
echo "  Master key: $(run "sed -n 's/^GREGAL_API_TOKEN=//p' .env" | tr -d '\r')"
echo "  Next step: set the model API keys (GUIDE.md, section 5)."
