#!/bin/sh
# Guided or non-interactive setup; delegates installation to install.sh.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
usage() {
 cat <<'EOF'
Ús: sudo sh setup.sh [--mode tunnel|lan|caddy] [--admin NOM] [--binary FITXER] [--domain DOMINI] [--email CORREU]
Sense opcions pregunta les dades. Amb --mode no pregunta res.
--admin és obligatori en una instal·lació nova; la contrasenya es genera.
En una actualització, utilitza el mateix mode i domini que ja tenies.
EOF
}
mode=""; admin=""; domain=""; email=""; binary=""; interactive=1
while [ "$#" -gt 0 ]; do
 case "$1" in
  --mode) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; mode=$2; interactive=0; shift 2 ;;
  --admin) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; admin=$2; shift 2 ;;
  --binary) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; binary=$2; shift 2 ;;
  --domain) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; domain=$2; shift 2 ;;
  --email) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; email=$2; shift 2 ;;
  -h|--help) usage; exit 0 ;;
  *) echo "Opció desconeguda: $1" >&2; usage >&2; exit 2 ;;
 esac
done
[ "$(id -u)" = 0 ] || { echo 'Executa: sudo sh setup.sh' >&2; exit 1; }
printf '\nGregal: instal·lació i actualització\n\n'
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Arquitectura no compatible.' >&2; exit 1 ;; esac
default="./gregal-linux-$arch"
if [ "$interactive" = 1 ]; then
 printf 'Binari de Gregal [%s]: ' "$default"; IFS= read -r binary; binary=${binary:-$default}
 printf '\nConnexió:\n  1) Cloudflare Tunnel o proxy HTTPS existent\n  2) Només xarxa local (HTTP sense xifrar)\n  3) Domini amb HTTPS automàtic (Caddy)\nOpció [1]: '
 IFS= read -r choice; case "${choice:-1}" in 1) mode=tunnel ;; 2) mode=lan ;; 3) mode=caddy ;; *) echo 'Opció desconeguda.' >&2; exit 1 ;; esac
 if [ "$mode" = caddy ]; then printf 'Domini (p. ex. gregal.exemple.cat): '; IFS= read -r domain; fi
fi
binary=${binary:-$default}
[ -f "$binary" ] || { echo "No existeix el binari: $binary" >&2; exit 1; }
case "$mode" in
 tunnel) set -- --domain localhost --no-caddy ;;
 lan) set -- --domain localhost --no-caddy --listen 0.0.0.0 ;;
 caddy) [ -n "$domain" ] || { echo 'Falta --domain.' >&2; exit 2; }; set -- --domain "$domain" ;;
 *) echo 'Indica --mode tunnel, lan o caddy.' >&2; exit 2 ;;
esac
if [ ! -f /var/lib/gregal/.config/gregal/config.yaml ]; then
 if [ "$interactive" = 1 ]; then printf 'Nom del primer compte administrador: '; IFS= read -r admin; fi
 [ -n "$admin" ] || { echo 'En una instal·lació nova cal --admin NOM.' >&2; exit 2; }
 set -- "$@" --user "$admin"
else
 [ -z "$admin" ] || echo "Instal·lació existent: --admin no modifica cap compte."
 echo 'Instal·lació existent: es conservaran els comptes, la clau i les dades.'
fi
[ -z "$email" ] || set -- "$@" --email "$email"
[ -f ./install.sh ] || { echo 'Falta install.sh al costat de setup.sh.' >&2; exit 1; }
chmod u+x "$binary"
sh ./install.sh "$@" --binary "$binary"
if [ "$mode" = tunnel ]; then echo 'Configura cloudflared perquè apunti a http://127.0.0.1:8097 i obre la URL HTTPS del túnel.'; fi
