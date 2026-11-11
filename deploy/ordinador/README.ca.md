# Ordinador personal: VM pilot (paquet B2)

Recepta reproduïble de l'ordinador persistent del pla v2 (§5 de `docs/android-assistent-experiencia-v2.ca.md`, revisat l'11/11). Hi ha dues peces:

- **Amfitrió, sempre encès i lleuger:** el servidor Gregal de sempre (`deploy/linux`): comptes, converses, estat durable, esdeveniments, avisos, gateway i webs publicades. Funciona encara que les VM estiguin apagades.
- **Ordinador, una VM per persona que s'encén quan cal:** escriptori gràfic lleuger, Chrome visible amb perfil persistent, visor noVNC només a loopback, volum persistent per al perfil i els fitxers, i l'agent mínim `gregal-ordinador`, que diu la seua salut i executa les eines que li envia l'amfitrió. A la VM **no** hi corre el servidor Gregal ni s'hi publica res.

Aquesta recepta no aprovisiona res, no obre cap port i no exposa VNC ni CDP. El pont autenticat del visor cap al mòbil és el paquet B3, i la publicació de webs és el B11 (a l'amfitrió).

## Peces

```
 Amfitrió (sempre encès)                         VM de la Carla (s'encén quan cal)
 ┌───────────────────────────────┐   xarxa      ┌──────────────────────────────────────────┐
 │ gregal.service 127.0.0.1:8097 │   privada    │ gregal-ordinador  10.66.0.2:8098 (Bearer) │
 │  ← túnel / Caddy (deploy/linux)│ ──────────▶ │   ├─ GREGAL_BROWSER_CDP=127.0.0.1:9222    │
 │ GREGAL_ORDINADORS=registre    │  (WireGuard, │   └─ eines: browser, bash, fitxers…       │
 │ ordinador-hipervisor.sh ──────┼─ Tailscale)  │ gregal-navegador  Chrome visible, CDP lo  │
 │   status | wake | stop        │              │   perfil: /srv/ordinador/perfil (volum)   │
 └───────────────┬───────────────┘              │ gregal-escriptori Xvnc :1 (socket Unix)   │
                 │ virsh / API del proveïdor    │ gregal-finestres  openbox                 │
                 ▼                              │ gregal-visor      noVNC 127.0.0.1:6080    │
           hipervisor ─────────────────────────▶│ /srv/ordinador: perfil/ fitxers/ …        │
                                                 └──────────────────────────────────────────┘
```

- Tot corre com l'usuari de sistema `gregal`, mai root.
- El VNC no té cap port TCP: Xvnc escolta en un socket Unix (`/run/gregal-ordinador/vnc.sock`, 0600) i websockify el tradueix a WebSocket a `127.0.0.1:6080`.
- Chrome escolta el CDP a `127.0.0.1:9222`. L'agent s'hi connecta i obre **una pestanya per feina**; quan la feina acaba, en tanca la pestanya, no el navegador.
- L'agent només escolta en una IP privada (es nega a arrencar amb `0.0.0.0` o una IP pública), exigeix el testimoni de 32 bytes i només accepta feina del seu propietari (`--owner`) i de sessions d'aquest compte. Les eines de l'amfitrió (vents, publicar, correu…) es deneguen; les de la VM treballen dins `/srv/ordinador/fitxers`.
- `setup.sh` comprova en acabar que cap port de l'ordinador (9222, 6080, 59xx) escolta fora de loopback; si en troba un, atura el visor, el navegador i l'agent, i falla.

## Com s'engega

### A la VM de cada persona

1. **Crea la VM** al hipervisor (2 vCPU, 4 GB de RAM, Debian 12 o Ubuntu 24.04 amd64). Ha de tenir una IP a la xarxa privada amb l'amfitrió (WireGuard, Tailscale o la xarxa interna del proveïdor). Sense ports oberts a Internet.
2. **Afegeix-hi un disc persistent** i munta'l a `/srv/ordinador`. Exemple amb un disc nou `/dev/vdb` (comprova el nom amb `lsblk` abans; `mkfs` esborra el disc):

   ```
   sudo mkfs.ext4 -L ordinador /dev/vdb
   sudo mkdir -p /srv/ordinador
   echo 'LABEL=ordinador /srv/ordinador ext4 defaults,nofail 0 2' | sudo tee -a /etc/fstab
   sudo mount /srv/ordinador
   ```

3. **Compila l'agent** al teu ordinador de desenvolupament i copia'l a la VM amb aquesta carpeta:

   ```
   GOOS=linux GOARCH=amd64 go build -o gregal-ordinador ./cmd/gregal-ordinador
   ```

4. **Prepara l'ordinador** (dins la VM, des d'aquesta carpeta):

   ```
   sudo sh setup.sh --owner carla --listen 10.66.0.2:8098 --binary ./gregal-ordinador --dry-run
   sudo sh setup.sh --owner carla --listen 10.66.0.2:8098 --binary ./gregal-ordinador
   ```

   `--owner` és el nom del compte a l'amfitrió. `--listen` és la IP privada de la VM. Opcions: `--id` (defecte `pc-<owner>`) i `--volume` (defecte `/srv/ordinador`).

5. **Inicia sessió als llocs** que vulguis que l'agent faci servir, des del visor (abans de B3, amb un túnel SSH, que no obre cap port):

   ```
   ssh -N -L 6080:127.0.0.1:6080 usuari@la-vm
   ```

   Obre `http://127.0.0.1:6080/vnc.html`. El login queda al perfil del volum i sobreviu a aturar la VM.

### A l'amfitrió

1. Gregal instal·lat amb `deploy/linux/setup.sh` i els comptes `carla` i `aleix` creats.
2. **Testimonis:** copia el de cada VM a `/etc/gregal/ordinadors/<id>.token` (0640, grup `gregal`) sense mostrar-lo; `setup.sh` en dona l'ordre.
3. **Hipervisor:** instal·la `ordinador-hipervisor.sh` a `/usr/local/lib/gregal/` amb els permisos de «Permisos de l'usuari `gregal`», més avall.
4. **Registre** a `/etc/gregal/ordinadors.json`:

   ```json
   {"computers": [
     {"id": "pc-carla", "owner": "carla", "agent_url": "http://10.66.0.2:8098",
      "token_file": "/etc/gregal/ordinadors/pc-carla.token",
      "control": ["/usr/local/lib/gregal/ordinador-hipervisor.sh", "gregal-carla"],
      "idle_minutes": 10, "idle_action": "suspend", "wake_timeout_seconds": 90},
     {"id": "pc-aleix", "owner": "aleix", "agent_url": "http://10.66.0.3:8098",
      "token_file": "/etc/gregal/ordinadors/pc-aleix.token",
      "control": ["/usr/local/lib/gregal/ordinador-hipervisor.sh", "gregal-aleix"]}
   ]}
   ```

   `idle_minutes` es pot canviar per compte. `agent_url` ha de ser una IP privada amb port; si no, l'amfitrió no carrega el registre i `/api/ordinador` respon 503 (mai una salut inventada).
5. **Activa'l** amb un drop-in del servei `gregal`, sense tocar la unitat de `deploy/linux`:

   ```
   sudo mkdir -p /etc/systemd/system/gregal.service.d
   printf '[Service]\nEnvironment=GREGAL_ORDINADORS=/etc/gregal/ordinadors.json\nEnvironment=LIBVIRT_DEFAULT_URI=qemu:///system\n' | sudo tee /etc/systemd/system/gregal.service.d/ordinadors.conf
   sudo systemctl daemon-reload && sudo systemctl restart gregal
   ```

   La unitat de `deploy/linux` té `ProtectSystem=strict`: si el hipervisor és local, comprova que l'usuari `gregal` pot arribar al socket de libvirt.

6. **Desa les VM a disc** un cop preparades (`virsh managedsave gregal-carla --running`): és el seu estat normal.
7. **Comprova-ho** amb la sessió de cada persona: `GET /api/ordinador` ha de dir `"health":"stopped"` i `"power":"saved"` al moment, sense despertar la VM. `POST /api/ordinador {"action":"wake"}` la reprèn (`starting` i després `running`) i `{"action":"suspend"}` la torna a desar.

## Política d'encesa: desat a disc per defecte

Poden passar hores entre sessions, i l'ordinador no ha de gastar res en buit.

- **Estat normal: desat a disc.** La memòria de la VM es desa al disc (libvirt `managedsave`) i la VM no ocupa ni CPU ni RAM. El perfil del navegador, les sessions iniciades i els fitxers es conserven. Si el hipervisor no sap desar, `"idle_action": "stop"` l'atura (arrencada en fred en encendre's).
- **S'encén només per feina que el necessita:** navegador, terminal o fitxers de l'ordinador (`EnsureRunning` amb el motiu, p. ex. `task:42 browser`), o quan la persona l'obre per mirar o prendre el control (`viewer`, `user`). Xatejar, llegir resultats, aprovar decisions, veure webs publicades i les comprovacions periòdiques (cerques, IMAP, ICS, connectors) corren a l'amfitrió i **no** l'encenen.
- **La salut no l'encén mai.** `GET /api/ordinador` només pregunta l'estat al hipervisor (`status`); desat o aturat → `"health":"stopped"` (amb `"power":"saved"` o `"stopped"`) al moment, sense parlar amb la VM.
- **Es desa sol** quan fa `idle_minutes` (10 per defecte, per compte; `-1` = mai) que no rep feina, no hi ha cap eina en curs (`Exec`, o `Hold` mentre un encàrrec fa diverses crides), ningú té el control ni el visor obert (`controlOwner` `none`) i no està arrencant. El detector d'inactivitat és a l'amfitrió (`ParkIdle`, cada minut), no a la VM: l'amfitrió és qui sap si hi ha feina. Abans de desar-lo, la feina en curs acaba (no es desa amb crides actives) o B4 la posa en `waiting_computer`.
- **Represa:** si una feina el necessita i està desat, l'encàrrec passa a `waiting_computer`, l'amfitrió l'encén i espera fins a `wake_timeout_seconds` (90 s per defecte) que l'agent digui `running`. Objectiu del pilot: ≤ 30 s des de desat. Si no hi arriba, `ErrComputerUnavailable` i la feina continua en espera, no falla.
- **Mesura:** cada encesa, desada i aturada queda a `<registre>.uso.jsonl` (o `meter_file` al registre) amb l'hora, el motiu i els segons encesos. `GET /api/ordinador` en dona el resum a `usage`: `onSince`, `onSeconds` totals, `wakes` i `lastWakeReason`.

### Interfície amb el hipervisor

L'amfitrió crida una ordre amb quatre verbs. `ordinador-hipervisor.sh` n'és la versió per a libvirt:

| Verb | libvirt | Sortida | Topall |
|---|---|---|---|
| `status` | `virsh domstate` + `virsh dominfo` (hi ha imatge desada?) | una paraula: `running`, `saved`, `stopped` o `starting` | 3 s |
| `wake` | `virsh start` (reprèn la imatge desada), `resume`, `dompmwakeup` | res; codi 0 si s'ha pogut demanar | 60 s |
| `suspend` | `virsh managedsave --running` | res | 60 s |
| `stop` | `virsh shutdown` (ACPI) | res | 60 s |

Per a un altre hipervisor n'hi ha prou amb un script amb els mateixos verbs:

- Proxmox: `qm status <vmid> --verbose` (`lock: suspended` → `saved`), `qm suspend <vmid> --todisk 1`, `qm start <vmid>` (reprèn), `qm shutdown <vmid>`.
- AWS: `aws ec2 stop-instances --hibernate` (cal la hibernació activada a la instància) com a `suspend` i `start-instances` com a `wake`; `pending` → `starting`.
- Hetzner Cloud: no desa la memòria; `"idle_action": "stop"` amb `hcloud server shutdown|poweron`.

### Permisos de l'usuari `gregal` a l'amfitrió

Només `virsh domstate/dominfo/start/resume/dompmwakeup/managedsave/shutdown` sobre els dominis dels ordinadors: el grup `libvirt` amb `LIBVIRT_DEFAULT_URI=qemu:///system`, o una clau SSH restringida (`command=` a `authorized_keys`) si el hipervisor és una altra màquina.

## Operació a la VM

| Què | Ordre |
|---|---|
| Estat | `systemctl status gregal-ordinador gregal-navegador gregal-visor gregal-escriptori` |
| Registres | `journalctl -u gregal-ordinador -u gregal-navegador -u gregal-visor -f` |
| Reiniciar el navegador (conserva el perfil) | `sudo systemctl restart gregal-navegador` |
| Còpia de seguretat | una instantània del volum `/srv/ordinador` al hipervisor |
| Esborrar les dades de la persona | aturar els serveis i esborrar `/srv/ordinador/perfil` i `/srv/ordinador/fitxers` |
| Canviar el testimoni | esborrar `/etc/gregal-ordinador/token`, tornar a executar `setup.sh`, copiar el nou a l'amfitrió i reiniciar `gregal` |

`Restart=always` torna a engegar l'escriptori, el navegador, el visor i l'agent si cauen. `BindsTo` fa que, si cau l'escriptori, el navegador i el visor es reiniciïn amb ell.

## Variables

| On | Variable | Valor | Per a què |
|---|---|---|---|
| Amfitrió | `GREGAL_ORDINADORS` | `/etc/gregal/ordinadors.json` | Registre d'ordinadors; sense ell, `/api/ordinador` comprova la mateixa màquina (Gregal d'escriptori) |
| VM (agent) | `GREGAL_BROWSER_CDP` | `127.0.0.1:9222` | Chrome supervisat, pestanya per feina |
| VM (agent) | `GREGAL_ORDINADOR_PROFILE` | `/srv/ordinador/perfil` | Perfil que es comprova (escrivible) |
| VM (agent) | `GREGAL_ORDINADOR_VNC` | `127.0.0.1:6080` | Comprovació del visor (capacitat `viewer`); fora de loopback no compta |
| VM (agent) | `GREGAL_ORDINADOR_OWNER` | el posa `--owner` | Només el propietari entra al Chrome supervisat |
| Mode local | `GREGAL_ORDINADOR_START_GRACE` | 30s | Temps en què un navegador que encara no respon és `starting` |

## Perfil del navegador fora de la VM

Al Gregal d'escriptori o a un servidor multiusuari sense ordinadors, on el Chrome l'engega el mateix Gregal, el perfil també és ara de la persona i no de cada feina:

- Mode local (sense comptes): el perfil base de sempre, `<config>/browser`.
- Servidor amb comptes: `<config>/browser/ordinadors/<clau del compte>`. Dos comptes mai comparteixen perfil ni procés de Chrome; les feines d'un mateix compte comparteixen el Chrome, cadascuna a la seua pestanya.
- Amb `GREGAL_BROWSER_CDP` (un Chrome extern), només el principal local o el compte de `GREGAL_ORDINADOR_OWNER` s'hi poden connectar: un sol Chrome té un sol perfil.

**Migració.** Abans, cada sessió tenia el seu perfil a `<config>/browser/sessions/<hash>`. La primera vegada que un compte obre el navegador, si el seu perfil nou encara no existeix, s'hi mou el de la seua sessió per defecte (on hi havia el login habitual). Mai es fusionen dos perfils i mai es mou el perfil d'un altre compte (l'origen es calcula a partir de la clau del mateix compte). Els perfils de les altres sessions es queden on són: no es pot saber quin login és el bo i barrejar-los podria creuar comptes. Quan hagis comprovat que el login funciona, es poden esborrar a mà (`<config>/browser/sessions/`). A la VM no cal cap migració: el perfil és nou, al volum.

## Què no s'ha pogut verificar sense una VM real

Tot el que hi ha en aquesta carpeta s'ha revisat i `setup.sh --dry-run` s'ha executat, però **no s'ha instal·lat en cap VM ni hipervisor**:

- Que els paquets i camins (`/usr/bin/Xvnc`, `/usr/bin/websockify`, `/usr/share/novnc`, `google-chrome-stable`) siguin exactament aquests a Debian 12 i Ubuntu 24.04, i que TigerVNC accepti `-rfbport -1`, `-rfbunixpath` i `-rfbunixmode` (cal ≥ 1.11).
- Que el Chrome visible arrenqui com a servei sense sessió d'usuari (D-Bus; el keyring s'evita amb `--password-store=basic`) i que el seu sandbox funcioni sota systemd.
- Que `bash` de l'agent funcioni amb bubblewrap sota `ProtectSystem=strict`.
- Que el login fet pel visor sobrevisqui a `managedsave` + `start`, a `shutdown` + `start` i a reiniciar el navegador.
- `ordinador-hipervisor.sh` contra un libvirt de debò: els textos exactes de `virsh domstate`/`dominfo`, els permisos de l'usuari `gregal`, i el temps real de represa des del disc (objectiu ≤ 30 s; depèn de la RAM i del disc) i d'arrencada en fred.
- Que Chrome, Xvnc i l'agent es recuperin bé després d'una represa (rellotge, connexions de xarxa caducades, CDP).
- La xarxa privada (WireGuard/Tailscale) entre l'amfitrió i la VM, i la comprovació final de ports (`ss`).
- Rendiment i cost: 2 vCPU / 4 GB és una estimació.

El codi sí que té proves amb un CDP, un agent i un hipervisor falsos: `go test ./internal/ordinador ./internal/tools -run 'Ordinador|OwnerChrome|BrowserOwner|Agent|Host|Registry|CheckPrivate'` i `go test ./internal/web -run Ordinador`.
