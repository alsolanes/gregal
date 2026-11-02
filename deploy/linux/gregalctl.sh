#!/bin/sh
# Local administration; account passwords are read from stdin, never argv.
set -eu
usage() {
 cat <<'EOF'
Ús: sudo sh gregalctl.sh ORDRE
  status                         Estat del servei
  accounts                       Llista els comptes
  invite                         Genera un codi d’un sol ús (30 minuts)
  account-create NOM             Crea un compte; contrasenya per stdin
  account-password NOM           Canvia la contrasenya d'un compte gestionat; per stdin
  account-delete NOM             Esborra un compte gestionat (les dades s'aparten)
  backup [DIRECTORI]             Còpia coherent i reinici si estava en marxa

Exemple interactiu (no escriu la contrasenya a l'historial):
  stty -echo; printf 'Contrasenya: '; read -r p; stty echo; printf '\n';
  printf '%s\n' "$p" | sudo sh gregalctl.sh account-create anna; unset p
account-password gestiona els comptes creats al panell o amb aquesta eina.
Per a l'administrador definit al config, consulta la guia del servidor; el
procediment antic no revoca automàticament les sessions existents.
EOF
}
[ "$#" -gt 0 ] || { usage; exit 2; }
case "$1" in -h|--help|help) usage; exit 0 ;; esac
[ "$(id -u)" = 0 ] || { echo 'Cal executar-ho amb sudo.' >&2; exit 1; }
command=$1; shift
case "$command" in
 status) [ "$#" = 0 ] || { usage >&2; exit 2; }; systemctl status gregal --no-pager ;;
 backup)
  [ "$#" -le 1 ] || { usage >&2; exit 2; }
  dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  running=0
  if systemctl is-active --quiet gregal; then running=1; systemctl stop gregal; fi
  restart() { if [ "$running" = 1 ]; then systemctl start gregal || { echo 'No s’ha pogut reiniciar el servei.' >&2; exit 1; }; running=0; fi; }
  trap 'restart' EXIT
  if [ "$#" = 1 ]; then sh "$dir/backup.sh" "$1"; else sh "$dir/backup.sh"; fi
  restart; trap - EXIT ;;
 accounts|invite|account-create|account-password|account-delete)
  case "$command:$#" in accounts:0|invite:0|account-create:1|account-password:1|account-delete:1) ;; *) usage >&2; exit 2 ;; esac
  command -v curl >/dev/null 2>&1 || { echo 'Cal curl.' >&2; exit 1; }
  command -v python3 >/dev/null 2>&1 || { echo 'Cal python3.' >&2; exit 1; }
  [ -r /etc/gregal/env ] || { echo 'No es troba /etc/gregal/env.' >&2; exit 1; }
  port=${GREGAL_CONTROL_PORT:-8097}; case "$port" in ''|*[!0-9]*) echo 'Port no vàlid.' >&2; exit 2 ;; esac
  token=$(sed -n 's/^GREGAL_API_TOKEN=//p' /etc/gregal/env)
  case "$token" in ''|*[!A-Za-z0-9_-]*) echo 'Clau del servidor no vàlida.' >&2; exit 1 ;; esac
  umask 077; tmp=$(mktemp -d /tmp/gregalctl.XXXXXX)
  trap 'rm -rf -- "$tmp"' EXIT
  trap 'exit 1' HUP INT TERM
  printf 'header = "Authorization: Bearer %s"\n' "$token" >"$tmp/curl.conf"; unset token
  url="http://127.0.0.1:$port/api/server/accounts"
  if [ "$command" = accounts ]; then
   code=$(curl --noproxy '*' --silent --show-error --max-time 30 --config "$tmp/curl.conf" -o "$tmp/result" -w '%{http_code}' "$url")
  elif [ "$command" = invite ]; then
   code=$(curl --noproxy '*' --silent --show-error --max-time 30 --config "$tmp/curl.conf" -X POST -o "$tmp/result" -w '%{http_code}' "http://127.0.0.1:$port/api/server/invitations")
  elif [ "$command" = account-delete ]; then
   python3 -c 'import json,sys; json.dump({"name":sys.argv[1],"action":"delete"},sys.stdout)' "$1" >"$tmp/body"
   code=$(curl --noproxy '*' --silent --show-error --max-time 30 --config "$tmp/curl.conf" -H 'Content-Type: application/json' --data-binary "@$tmp/body" -o "$tmp/result" -w '%{http_code}' "$url")
  else
   action=create; [ "$command" = account-password ] && action=reset
   python3 -c 'import json,sys; p=sys.stdin.readline(1026).rstrip("\r\n"); (len(p)>=8 and len(p.encode())<=1024) or sys.exit("La contrasenya ha de tenir entre 8 caràcters i 1024 bytes."); json.dump({"name":sys.argv[1],"action":sys.argv[2],"password":p},sys.stdout)' "$1" "$action" >"$tmp/body"
   code=$(curl --noproxy '*' --silent --show-error --max-time 30 --config "$tmp/curl.conf" -H 'Content-Type: application/json' --data-binary "@$tmp/body" -o "$tmp/result" -w '%{http_code}' "$url")
  fi
  cat "$tmp/result"; printf '\n'
  case "$code" in 2??) ;; *) echo "Error del servidor (HTTP $code)." >&2; exit 1 ;; esac ;;
 *) usage >&2; exit 2 ;;
esac
