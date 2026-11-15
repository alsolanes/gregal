# Contenidor local del navegador persistent de Gregal

Aquest contenidor aporta Chrome visible + perfil persistent + noVNC al Gregal d'un sol usuari. No obre ports a la LAN/Internet: el CDP i noVNC es publiquen a `127.0.0.1` al host; l'app accedeix al noVNC només mitjançant `/ordinador/visor/` autenticat per Gregal.

L'aïllament del renderer de Chrome no es pot activar amb el perfil seccomp per defecte de Docker, que bloqueja la creació dels user namespaces necessaris. Per això Chrome corre com a UID 1000, sense capabilities de Linux, dins d'un contenidor dedicat amb només el perfil muntat. CDP i noVNC només es publiquen a loopback. És més aïllat que posar el navegador amb contrasenyes dins del procés del servidor Gregal, però no equival a una VM dedicada.

## Build

```sh
docker build -t gregal-browser:local /home/aleix/gregal-live/deploy/ordinador/container
```

## Run (host Aleix)

```sh
mkdir -p -m 0700 /home/aleix/.local/share/gregal/ordinador/profile
docker network create gregal-browser

docker run -d --name gregal-browser \
  --restart unless-stopped \
  --network gregal-browser \
  --memory 3g --cpus 2 --shm-size 1g \
  --cap-drop ALL --security-opt no-new-privileges \
  -p 127.0.0.1:6080:6080 -p 127.0.0.1:9222:9223 \
  -v /home/aleix/.local/share/gregal/ordinador/profile:/data/perfil \
  gregal-browser:local
```

Els ports del visor i CDP només s'exposen a loopback. No els publiquis a `0.0.0.0` ni posis aquest contenidor darrere d'un proxy públic directament.

## Servei Gregal

Afegeix un drop-in d'usuari a `gregal-web.service`:

```ini
[Service]
Environment=GREGAL_BROWSER_CDP=127.0.0.1:9222
Environment=GREGAL_ORDINADOR_PROFILE=/home/aleix/.local/share/gregal/ordinador/profile
Environment=GREGAL_ORDINADOR_VNC=127.0.0.1:6080
Environment=GREGAL_ORDINADOR_VISOR=127.0.0.1:6080
```

Aplica el drop-in i reinicia Gregal:

```sh
systemctl --user daemon-reload
systemctl --user restart gregal-web
```

`GET /api/ordinador` ha d'indicar `health=running`, `browserMode=attach` i les capacitats `browser` i `viewer`; les comprovacions de navegador i visor han de passar. Prova `POST /api/ordinador/viewer`, obre l'URL d'un sol ús i comprova que el WebSocket autenticat fa upgrade i transporta el banner VNC `RFB`. A Android, obre **Vents → Ordinador → Pren el control** i inicia sessió als webs tu mateix. L'inici de sessió queda només al perfil muntat de Chrome. Després demana a Gregal que revisi les dades i t'aconselli; qualsevol venda o altra acció irreversible requereix aprovació explícita.
