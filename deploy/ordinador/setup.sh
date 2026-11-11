#!/bin/sh
# setup.sh — prepara la VM d'una persona com a «ordinador personal» del pilot
# (pla v2 §5, paquet B2): escriptori gràfic lleuger, Chrome visible amb perfil
# persistent, visor noVNC NOMÉS a loopback, volum persistent per al perfil i
# els fitxers, i l'agent mínim `gregal-ordinador` supervisat per systemd.
#
# La VM NO és el servidor Gregal. Els comptes, les converses, els avisos i
# les webs són a l'amfitrió, sempre encès. L'amfitrió encén aquesta VM quan
# una feina la necessita, li envia les eines per un canal privat autenticat
# i la desa a disc (suspensió) quan no té feina: és el seu estat normal.
#
#   sudo sh setup.sh --owner carla --listen 10.66.0.2:8098 --binary ./gregal-ordinador [--id pc-carla] [--volume /srv/ordinador] [--dry-run]
#
# Requisits (README.ca.md): Debian 12 o Ubuntu 22.04/24.04 amd64 amb
# systemd; el volum persistent ja muntat a --volume; una xarxa privada amb
# l'amfitrió (WireGuard/Tailscale o la xarxa interna del proveïdor) i --listen
# en una IP d'aquesta xarxa.
#
# Què NO fa: no obre cap port, no toca el tallafoc, no exposa VNC ni CDP, no
# crea túnels ni instal·la el servidor Gregal. Es pot tornar a executar: no
# esborra el perfil, els fitxers ni el testimoni.
set -eu

OWNER=""
LISTEN=""
BIN=""
VOLUME=/srv/ordinador
ID=""
DRY=0
DISPLAY_NUM=1
CDP_PORT=9222
VNC_WEB_PORT=6080
GEOMETRY=1280x900
TOKEN_DIR=/etc/gregal-ordinador

die() { echo "ordinador/setup.sh: $*" >&2; exit 1; }
info() { echo "==> $*"; }
run() {
	if [ "$DRY" = 1 ]; then
		echo "[dry-run] $*"
	else
		"$@"
	fi
}
# write FITXER: el contingut ve per stdin.
write() {
	if [ "$DRY" = 1 ]; then
		echo "[dry-run] escriuria $1:"
		sed 's/^/    /'
	else
		cat >"$1"
	fi
}

usage() {
	cat <<'EOF'
Ús: sudo sh setup.sh --owner NOM --listen IP:PORT --binary FITXER [--id ID] [--volume DIR] [--dry-run]
  --owner NOM      compte de Gregal (a l'amfitrió) propietari d'aquest ordinador
  --listen IP:PORT on escolta l'agent: una IP de la xarxa privada amb l'amfitrió
                   (mai 0.0.0.0 ni una IP pública)
  --binary FITXER  gregal-ordinador per a linux/amd64
                   (GOOS=linux GOARCH=amd64 go build -o gregal-ordinador ./cmd/gregal-ordinador)
  --id ID          identificador (defecte: pc-<owner>); el mateix que al registre de l'amfitrió
  --volume DIR     volum persistent ja muntat (defecte: /srv/ordinador)
  --dry-run        mostra el que faria sense canviar res
EOF
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--owner) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; OWNER=$2; shift 2 ;;
	--listen) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; LISTEN=$2; shift 2 ;;
	--binary) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; BIN=$2; shift 2 ;;
	--volume) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; VOLUME=$2; shift 2 ;;
	--id) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; ID=$2; shift 2 ;;
	--dry-run) DRY=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; exit 2 ;;
	esac
done

[ -n "$OWNER" ] && [ -n "$LISTEN" ] && [ -n "$BIN" ] || { usage >&2; die "calen --owner, --listen i --binary"; }
case "$OWNER" in *[!A-Za-z0-9._@-]*) die "--owner: només lletres, xifres i . _ @ -" ;; esac
case "$VOLUME" in /*) ;; *) die "--volume ha de ser un camí absolut" ;; esac
[ -n "$ID" ] || ID="pc-$OWNER"
case "$ID" in *[!A-Za-z0-9._-]*) die "--id: només lletres, xifres i . _ -" ;; esac
# L'agent mateix rebutja adreces públiques; aquí es para abans d'instal·lar.
LHOST=${LISTEN%:*}
case "$LHOST" in
127.* | 10.* | 192.168.* | 172.1[6-9].* | 172.2[0-9].* | 172.3[01].* | 100.6[4-9].* | 100.[7-9][0-9].* | 100.1[01][0-9].* | 100.12[0-7].*) ;;
*) die "--listen $LISTEN: cal una IP privada (10/8, 172.16/12, 192.168/16, 100.64/10) o de loopback" ;;
esac

if [ "$DRY" = 0 ]; then
	[ "$(id -u)" = 0 ] || die "executa-ho amb sudo"
	command -v systemctl >/dev/null 2>&1 || die "cal systemd"
	command -v apt-get >/dev/null 2>&1 || die "cal Debian o Ubuntu (apt-get)"
	[ "$(uname -m)" = x86_64 ] || die "Chrome per a Linux només hi és per a amd64"
	[ -f "$BIN" ] || die "no existeix el binari: $BIN"
	[ -d "$VOLUME" ] || die "$VOLUME no existeix: munta-hi el volum persistent (README.ca.md)"
	if command -v mountpoint >/dev/null 2>&1 && ! mountpoint -q "$VOLUME"; then
		echo "Avís: $VOLUME no és un punt de muntatge; el perfil i els fitxers es perdran si es refà el disc de la VM." >&2
	fi
fi

# 1. Paquets: X virtual amb VNC (TigerVNC), gestor de finestres lleuger,
#    noVNC + websockify, fonts, eines bàsiques per al terminal i Chrome del
#    repositori signat de Google.
info "paquets de l'escriptori"
run apt-get update
run env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
	tigervnc-standalone-server openbox novnc python3-websockify \
	fonts-noto fonts-noto-color-emoji dbus-x11 ca-certificates curl gnupg iproute2 \
	git bubblewrap
if ! command -v google-chrome-stable >/dev/null 2>&1; then
	info "Google Chrome (repositori signat)"
	run install -d -m 0755 /etc/apt/keyrings
	if [ "$DRY" = 1 ]; then
		echo "[dry-run] curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor -o /etc/apt/keyrings/google-chrome.gpg"
	else
		curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor --yes -o /etc/apt/keyrings/google-chrome.gpg
	fi
	echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main" |
		write /etc/apt/sources.list.d/google-chrome.list
	run apt-get update
	run env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends google-chrome-stable
fi

# 2. Usuari de sistema, binari i testimoni compartit amb l'amfitrió.
info "usuari gregal i agent gregal-ordinador"
if [ "$DRY" = 1 ] || ! id gregal >/dev/null 2>&1; then
	run useradd --system --create-home --home-dir /var/lib/gregal --shell /usr/sbin/nologin gregal
fi
run install -m 0755 "$BIN" /usr/local/bin/gregal-ordinador
run install -d -o root -g gregal -m 0750 "$TOKEN_DIR"
if [ "$DRY" = 1 ]; then
	echo "[dry-run] generaria $TOKEN_DIR/token (32 bytes aleatoris en hex, 0640 root:gregal) si no existeix"
elif [ ! -s "$TOKEN_DIR/token" ]; then
	(umask 027 && head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' >"$TOKEN_DIR/token")
	chgrp gregal "$TOKEN_DIR/token"
fi

# 3. Volum persistent: perfil del navegador i carpeta de treball/resultats.
info "volum persistent a $VOLUME"
run install -d -o gregal -g gregal -m 0700 "$VOLUME" "$VOLUME/perfil" "$VOLUME/fitxers" "$VOLUME/descarregues"

# 4. Unitats systemd. Totes com a `gregal`, mai root. El VNC és un socket
#    Unix 0600 (cap port TCP); websockify i el CDP escolten a 127.0.0.1;
#    només l'agent escolta a la xarxa privada, i amb testimoni.
info "unitats systemd"
write /etc/systemd/system/gregal-escriptori.service <<EOF
[Unit]
Description=Gregal ordinador: escriptori X amb VNC (socket Unix, sense port TCP)
After=network.target

[Service]
User=gregal
Group=gregal
RuntimeDirectory=gregal-ordinador
RuntimeDirectoryMode=0700
Environment=HOME=/var/lib/gregal
ExecStart=/usr/bin/Xvnc :$DISPLAY_NUM -geometry $GEOMETRY -depth 24 -rfbport -1 -rfbunixpath /run/gregal-ordinador/vnc.sock -rfbunixmode 0600 -SecurityTypes None -nolisten tcp -AlwaysShared
Restart=always
RestartSec=3
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
EOF

write /etc/systemd/system/gregal-finestres.service <<EOF
[Unit]
Description=Gregal ordinador: gestor de finestres (openbox)
BindsTo=gregal-escriptori.service
After=gregal-escriptori.service

[Service]
User=gregal
Group=gregal
Environment=HOME=/var/lib/gregal
Environment=DISPLAY=:$DISPLAY_NUM
ExecStart=/usr/bin/openbox
Restart=always
RestartSec=3
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
EOF

# Chrome visible a l'escriptori, perfil al volum. El CDP queda a loopback i
# l'agent s'hi connecta (GREGAL_BROWSER_CDP). --user-data-dir propi és
# obligatori: Chrome no accepta depuració remota amb el perfil per defecte.
write /etc/systemd/system/gregal-navegador.service <<EOF
[Unit]
Description=Gregal ordinador: Chrome visible amb perfil persistent
BindsTo=gregal-escriptori.service
After=gregal-escriptori.service gregal-finestres.service
RequiresMountsFor=$VOLUME

[Service]
User=gregal
Group=gregal
Environment=HOME=/var/lib/gregal
Environment=DISPLAY=:$DISPLAY_NUM
ExecStart=/usr/bin/google-chrome-stable --user-data-dir=$VOLUME/perfil --remote-debugging-address=127.0.0.1 --remote-debugging-port=$CDP_PORT --no-first-run --no-default-browser-check --disable-sync --password-store=basic --disable-features=Translate,MediaRouter --window-position=0,0 --window-size=$(echo $GEOMETRY | tr x ,) --start-maximized about:blank
Restart=always
RestartSec=3
# Chrome fa servir el seu propi sandbox d'espais de noms: no s'hi posa
# NoNewPrivileges perquè el desactivaria.

[Install]
WantedBy=multi-user.target
EOF

# noVNC: websockify tradueix WebSocket ↔ socket Unix del VNC i serveix el
# client web, NOMÉS a 127.0.0.1. El pont autenticat des de l'amfitrió és B3.
write /etc/systemd/system/gregal-visor.service <<EOF
[Unit]
Description=Gregal ordinador: visor noVNC (només loopback)
BindsTo=gregal-escriptori.service
After=gregal-escriptori.service

[Service]
User=gregal
Group=gregal
ExecStart=/usr/bin/websockify --web /usr/share/novnc 127.0.0.1:$VNC_WEB_PORT --unix-target=/run/gregal-ordinador/vnc.sock
Restart=always
RestartSec=3
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
EOF

# L'agent: salut i eines per a l'amfitrió. Les eines de terminal corren
# aquí, dins la VM dedicada, amb el sandbox de sempre (bubblewrap).
write /etc/systemd/system/gregal-ordinador.service <<EOF
[Unit]
Description=Gregal ordinador: agent ($ID, $OWNER)
Wants=gregal-navegador.service gregal-visor.service network-online.target
After=gregal-navegador.service network-online.target
RequiresMountsFor=$VOLUME

[Service]
User=gregal
Group=gregal
WorkingDirectory=$VOLUME/fitxers
Environment=HOME=/var/lib/gregal
Environment=GREGAL_BROWSER_CDP=127.0.0.1:$CDP_PORT
Environment=GREGAL_ORDINADOR_VNC=127.0.0.1:$VNC_WEB_PORT
Environment=GREGAL_ORDINADOR_PROFILE=$VOLUME/perfil
ExecStart=/usr/local/bin/gregal-ordinador --listen $LISTEN --id $ID --owner $OWNER --files $VOLUME/fitxers --token-file $TOKEN_DIR/token
Restart=always
RestartSec=3
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$VOLUME /var/lib/gregal

[Install]
WantedBy=multi-user.target
EOF

run systemctl daemon-reload
for u in gregal-escriptori gregal-finestres gregal-navegador gregal-visor gregal-ordinador; do
	run systemctl enable --now "$u"
done

# 5. Comprovació: res de l'ordinador escolta fora de loopback, excepte
#    l'agent a la IP privada indicada.
if [ "$DRY" = 0 ]; then
	info "comprovant que VNC/CDP no surten de la màquina"
	sleep 5
	if ss -ltnH | awk '{print $4}' | grep -E ":($CDP_PORT|$VNC_WEB_PORT|59[0-9][0-9])\$" | grep -vE '^(127\.0\.0\.1|\[::1\]):'; then
		systemctl stop gregal-visor gregal-navegador gregal-ordinador
		die "hi ha un port de l'ordinador escoltant fora de loopback (a dalt): s'han aturat el visor, el navegador i l'agent"
	fi
	for u in gregal-escriptori gregal-finestres gregal-navegador gregal-visor gregal-ordinador; do
		printf '  %-20s %s\n' "$u" "$(systemctl is-active "$u" || true)"
	done
fi

cat <<EOF

Ordinador «$ID» preparat per a $OWNER.
  Agent:     http://$LISTEN (xarxa privada, amb testimoni)
  Testimoni: $TOKEN_DIR/token — copia'l a l'amfitrió sense mostrar-lo, p. ex.:
             ssh la-vm sudo cat $TOKEN_DIR/token | ssh l-amfitrio 'sudo install -m 0640 -g gregal /dev/stdin /etc/gregal/ordinadors/$ID.token'
  Perfil:    $VOLUME/perfil
  Fitxers:   $VOLUME/fitxers
  Visor:     127.0.0.1:$VNC_WEB_PORT (només loopback; README.ca.md)
  Registres: journalctl -u gregal-ordinador -u gregal-navegador -u gregal-visor -f
Afegeix l'entrada «$ID» al registre de l'amfitrió (README.ca.md, «A l'amfitrió»).
EOF
