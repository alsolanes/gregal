#!/bin/sh
# install.sh — Gregal com a servidor sempre encès en una màquina Linux, obert
# a internet pel teu domini amb HTTPS.
#
#   sudo sh install.sh --domain vents.exemple.cat --binary ./gregal-linux-amd64
#
# Què fa (es pot tornar a executar: no esborra res):
#   1. Usuari de sistema `gregal` amb casa a /var/lib/gregal (dades, config,
#      projectes). Gregal no corre mai com a root.
#   2. Copia el binari a /usr/local/bin/gregal.
#   3. Una clau d'accés (o, amb --user, un compte amb contrasenya). En
#      acabar mostra el que cal escriure a l'app: adreça, port i clau.
#   4. Servei systemd `gregal` escoltant NOMÉS a 127.0.0.1:8097, amb
#      reinici automàtic i l'aïllament de systemd (no pot escriure fora de
#      /var/lib/gregal).
#   5. Caddy al davant: certificat HTTPS automàtic (Let's Encrypt) per al
#      domini, HSTS i els fluxos en directe (SSE) sense memòria intermèdia.
#
# Abans: el domini ha d'apuntar (registre A/AAAA) a aquesta màquina i els
# ports 80 i 443 han d'estar oberts. Requereix Debian/Ubuntu amb systemd
# (per a altres distribucions, instal·la Caddy a mà i fes servir --no-caddy).
set -eu

DOMAIN=""
BIN=""
ADMIN=""
PASSWORD="${GREGAL_ADMIN_PASSWORD:-}"
PORT=8097
EMAIL=""
TLS_INTERNAL=0
NO_CADDY=0
HOME_DIR=/var/lib/gregal

die() { echo "install.sh: $*" >&2; exit 1; }
info() { echo "==> $*"; }

usage() {
	cat <<'EOF'
Ús: sudo sh install.sh --domain DOMINI --binary FITXER [opcions]

  --domain DOMINI     domini públic (p. ex. vents.exemple.cat)
  --binary FITXER     binari de Gregal per a Linux (gregal-linux-amd64 o -arm64)
  --user NOM          a més de la clau, un compte amb contrasenya (opcional)
  --email ADREÇA      correu per a Let's Encrypt (avisos de caducitat)
  --port PORT         port intern (defecte: 8097, només a 127.0.0.1)
  --no-caddy          no instal·lis ni configuris Caddy (ja tens un proxy)
  --tls-internal      certificat propi de Caddy en lloc de Let's Encrypt
                      (proves sense domini real)

Al final mostra l'adreça, el port i la clau per a l'app. Amb --user, la
contrasenya es genera i es mostra (o tria-la amb GREGAL_ADMIN_PASSWORD).
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--domain) DOMAIN="${2:-}"; shift 2 ;;
	--binary) BIN="${2:-}"; shift 2 ;;
	--user) ADMIN="${2:-}"; shift 2 ;;
	--email) EMAIL="${2:-}"; shift 2 ;;
	--port) PORT="${2:-}"; shift 2 ;;
	--no-caddy) NO_CADDY=1; shift ;;
	--tls-internal) TLS_INTERNAL=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "opció desconeguda: $1" ;;
	esac
done

[ "$(id -u)" = 0 ] || die "cal executar-lo com a root (sudo)"
[ -n "$DOMAIN" ] || { usage >&2; die "falta --domain"; }
[ -n "$BIN" ] && [ -f "$BIN" ] || { usage >&2; die "falta --binary (o el fitxer no existeix)"; }
command -v systemctl >/dev/null 2>&1 || die "cal systemd"
[ -z "$ADMIN" ] || case "$ADMIN" in *[!A-Za-z0-9._-]*) die "--user: només lletres, xifres, punt, guió i guió baix" ;; esac
case "$PORT" in *[!0-9]* | "") die "--port ha de ser un número" ;; esac
case "$DOMAIN" in *[!A-Za-z0-9.-]*) die "--domain: només el nom (sense https:// ni camins)" ;; esac

"$BIN" --version >/dev/null 2>&1 || die "$BIN no s'executa en aquesta màquina (arquitectura equivocada?)"

# 1. Usuari de sistema.
if ! id gregal >/dev/null 2>&1; then
	info "creant l'usuari de sistema gregal ($HOME_DIR)"
	useradd --system --create-home --home-dir "$HOME_DIR" --shell /usr/sbin/nologin gregal
fi
mkdir -p "$HOME_DIR/projects"
chown -R gregal:gregal "$HOME_DIR"
chmod 0750 "$HOME_DIR"

asgregal() { runuser -u gregal -- env HOME="$HOME_DIR" "$@"; }

# 2. Binari. Si el servei corre, s'atura abans de substituir-lo.
info "instal·lant $(basename "$BIN") a /usr/local/bin/gregal"
if systemctl is-active --quiet gregal 2>/dev/null; then
	systemctl stop gregal
fi
install -m 0755 "$BIN" /usr/local/bin/gregal

# 3. Config i compte d'administrador.
CFG="$HOME_DIR/.config/gregal/config.yaml"
if [ ! -f "$CFG" ]; then
	info "creant el config ($CFG)"
	(cd "$HOME_DIR/projects" && asgregal /usr/local/bin/gregal init --config="$CFG" >/dev/null) ||
		die "no s'ha pogut crear $CFG"
fi
NEW_ADMIN=0
if [ -n "$ADMIN" ] && ! grep -q '^users:' "$CFG"; then
	info "afegint el compte $ADMIN al config"
	cat >>"$CFG" <<EOF

# Comptes que poden entrar per internet (usuari i contrasenya). Les
# contrasenyes no van aquí: es desen xifrades amb \`gregal --passwd NOM\`.
users:
  $ADMIN:
    admin: true
    roots:
      - $HOME_DIR/projects
    home: $HOME_DIR/projects
EOF
	chown gregal:gregal "$CFG"
	chmod 0600 "$CFG"
	NEW_ADMIN=1
fi
SHOW_PASSWORD=0
if [ -n "$ADMIN" ] && { [ "$NEW_ADMIN" = 1 ] || [ -n "$PASSWORD" ]; }; then
	if [ -z "$PASSWORD" ]; then
		PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -d '/+=\n' | cut -c1-24)
		SHOW_PASSWORD=1
	else
		SHOW_PASSWORD=0
	fi
	printf '%s\n' "$PASSWORD" | asgregal /usr/local/bin/gregal --config "$CFG" --passwd "$ADMIN" >/dev/null 2>&1 ||
		die "no s'ha pogut desar la contrasenya de $ADMIN"
fi

# Clau d'accés (la que s'escriu a l'app) i adreça pública, fora del config
# i de la línia d'ordres (no surten a `ps`). Es conserva entre instal·lacions.
mkdir -p /etc/gregal
ENVF=/etc/gregal/env
KEY=""
[ -f "$ENVF" ] && KEY=$(sed -n 's/^GREGAL_API_TOKEN=//p' "$ENVF")
if [ -z "$KEY" ]; then
	KEY=$(LC_ALL=C tr -dc '23456789abcdefghjkmnpqrstuvwxyz' </dev/urandom | head -c 20 | sed 's/.\{5\}/&-/g; s/-$//')
fi
PUBLIC_URL="https://$DOMAIN"
[ "$NO_CADDY" = 1 ] && PUBLIC_URL=""
umask 077
printf 'GREGAL_API_TOKEN=%s\nGREGAL_PUBLIC_URL=%s\n' "$KEY" "$PUBLIC_URL" >"$ENVF"
umask 022

# 4. Servei systemd.
info "configurant el servei gregal (127.0.0.1:$PORT)"
cat >/etc/systemd/system/gregal.service <<EOF
[Unit]
Description=Gregal (servidor dels vents)
After=network-online.target
Wants=network-online.target

[Service]
User=gregal
Group=gregal
WorkingDirectory=$HOME_DIR/projects
Environment=HOME=$HOME_DIR
# Les claus que entris al web (models, Exa) es desen a l'auth.yaml del servidor.
Environment=GREGAL_PERSIST_KEYS=1
EnvironmentFile=/etc/gregal/env
ExecStart=/usr/local/bin/gregal --serve --addr 127.0.0.1 --port $PORT --config $CFG
Restart=on-failure
RestartSec=5
# Aïllament: només pot escriure a la seva casa.
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$HOME_DIR
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
LockPersonality=yes

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now gregal >/dev/null 2>&1 || systemctl restart gregal

# Espera que respongui: sense credencials, /api/me ha de dir 401.
i=0
code=""
while [ $i -lt 30 ]; do
	code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/api/me" 2>/dev/null || true)
	[ "$code" = 401 ] && break
	i=$((i + 1))
	sleep 1
done
[ "$code" = 401 ] || die "el servei no respon com cal (codi «$code»): journalctl -u gregal -n 50"

# 5. Caddy (HTTPS).
if [ "$NO_CADDY" = 0 ]; then
	if ! command -v caddy >/dev/null 2>&1; then
		command -v apt-get >/dev/null 2>&1 || die "instal·la Caddy a mà (https://caddyserver.com/docs/install) i torna-ho a provar"
		info "instal·lant Caddy (repositori oficial)"
		apt-get update -qq
		apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl gnupg >/dev/null
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' >/etc/apt/sources.list.d/caddy-stable.list
		apt-get update -qq
		apt-get install -y -qq caddy >/dev/null
	fi
	mkdir -p /etc/caddy
	TLS_LINE=""
	[ "$TLS_INTERNAL" = 1 ] && TLS_LINE="	tls internal"
	[ -n "$EMAIL" ] && [ "$TLS_INTERNAL" = 0 ] && TLS_LINE="	tls $EMAIL"
	cat >/etc/caddy/gregal.caddy <<EOF
# Gregal: generat per deploy/linux/install.sh
$DOMAIN {
$TLS_LINE
	encode zstd gzip
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		-Server
	}
	reverse_proxy 127.0.0.1:$PORT {
		# Els vents i l'agent responen en directe (SSE): res de memòria
		# intermèdia, i sense tallar respostes llargues.
		flush_interval -1
		transport http {
			read_timeout 0
		}
	}
}
EOF
	CADDYFILE=/etc/caddy/Caddyfile
	if [ ! -f "$CADDYFILE" ] || grep -q 'root \* /usr/share/caddy' "$CADDYFILE"; then
		# El Caddyfile d'exemple del paquet (una pàgina estàtica al :80).
		[ -f "$CADDYFILE" ] && cp "$CADDYFILE" "$CADDYFILE.abans-gregal"
		echo "import /etc/caddy/gregal.caddy" >"$CADDYFILE"
	elif ! grep -q 'import /etc/caddy/gregal.caddy' "$CADDYFILE"; then
		printf '\nimport /etc/caddy/gregal.caddy\n' >>"$CADDYFILE"
	fi
	caddy validate --config "$CADDYFILE" --adapter caddyfile >/dev/null 2>&1 || die "el Caddyfile no és vàlid: caddy validate --config $CADDYFILE"
	systemctl enable --now caddy >/dev/null 2>&1 || true
	systemctl reload caddy 2>/dev/null || systemctl restart caddy
	if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q 'Status: active'; then
		ufw allow 80/tcp >/dev/null && ufw allow 443/tcp >/dev/null
	fi
fi

echo
echo "Gregal ja corre. A l'app del mòbil, «Connecta't a un servidor», escriu:"
if [ "$NO_CADDY" = 0 ]; then
	echo "  Adreça:  https://$DOMAIN"
	echo "  Port:    (no cal)"
else
	echo "  Adreça:  la del teu proxy HTTPS (el servei és a http://127.0.0.1:$PORT)"
fi
echo "  Clau:    $KEY"
if [ -n "$ADMIN" ]; then
	echo
	echo "També pots entrar amb l'usuari $ADMIN."
	if [ "$SHOW_PASSWORD" = 1 ]; then
		echo "  Contrasenya: $PASSWORD (desa-la ara: no es torna a mostrar)"
	fi
fi
echo
echo "La clau és a /etc/gregal/env (només root). Al navegador, la mateixa"
echo "adreça et demanarà la clau; a Configuració posa la clau del proveïdor de"
echo "models (i la d'Exa per a la cerca web)."
echo "Registre del servei: journalctl -u gregal -f"
