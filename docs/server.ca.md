# Servidor propi

L'app d'Android funciona sola, dins del mòbil. Si vols que els vents facin
feines llargues (webs, anàlisis, equips sencers) que continuïn encara que
tanquis l'app, engega Gregal en un ordinador o servidor i connecta-hi el mòbil.
A dalt de la pantalla dels vents tens l'interruptor **📱 Al mòbil · Servidor**:
un toc i canvies. Cada lloc té els seus vents i converses.

Connectar-hi el mòbil sempre és el mateix: **adreça, port i clau**. El servidor
t'ho diu en engegar-se.

## Instal·lació guiada (Debian/Ubuntu)

Si tens el paquet preparat per a Linux x86-64, copia'l al servidor,
descomprimeix-lo i engega l'assistent:

```sh
tar -xzf gregal-servidor-dev-linux-amd64.tar.gz
cd gregal-servidor-dev
sudo sh setup.sh
```

Alternativament, copia al servidor el binari Linux de la seva arquitectura i els fitxers
`deploy/linux/setup.sh`, `install.sh`, `gregalctl.sh` i `backup.sh` en una mateixa carpeta.
Des d'aquella carpeta executa:

```sh
sudo sh setup.sh
```

Per a un agent amb `sudo` no interactiu, la mateixa instal·lació es pot fer
sense preguntes (tria `tunnel`, `lan` o `caddy`):

```sh
sudo -n sh setup.sh --mode tunnel --admin aleix
```

Per a `caddy`, afegeix `--domain gregal.exemple.cat`. En una actualització
repeteix l'ordre amb el mateix mode i domini; `--admin` no modifica el compte
existent. El primer compte rep una contrasenya generada que es mostra a la
sortida: guarda-la en un lloc segur. `sudo -n` només funciona si l'agent ja té
permís per executar sudo sense interacció.

L'instal·lador afegeix `gregalctl` al PATH. Des del servidor:

```sh
sudo -n gregalctl status
sudo -n gregalctl accounts
sudo -n gregalctl invite  # codi d’un sol ús, vàlid durant 30 minuts
printf '%s\n' "$CONTRASENYA" | sudo -n gregalctl account-create anna
printf '%s\n' "$CONTRASENYA_NOVA" | sudo -n gregalctl account-password anna
sudo -n gregalctl backup
```

Les contrasenyes es llegeixen de l'entrada estàndard; les variables de
l'exemple s'han d'obtenir d'un canal segur, sense posar valors literals a
l'historial. `account-password` canvia comptes creats amb el panell o el
terminal i invalida les seves sessions. L'administrador inicial, definit al
config, utilitza l'ordre següent i reinicia el servei; aquesta via antiga no
revoca automàticament les sessions existents:

```sh
printf '%s\n' "$CONTRASENYA_NOVA" | sudo -n -u gregal env HOME=/var/lib/gregal gregal --config /var/lib/gregal/.config/gregal/config.yaml --passwd NOM
sudo -n systemctl restart gregal
```

Aquesta ordre assumeix les rutes estàndard de l'instal·lador. `gregalctl backup` atura el
servei mentre fa la còpia i el torna a engegar si abans estava actiu.

L'assistent pregunta pel binari, pel tipus de connexió i, en una instal·lació
nova, pel nom de l'administrador. Crea una contrasenya inicial i la mostra una
sola vegada. Amb cloudflared, cal tenir el túnel configurat a part i dirigir-lo
a `http://127.0.0.1:8097`; l'assistent no instal·la cloudflared. L'opció de
xarxa local usa HTTP sense xifrar: fes-la servir només en una xarxa de
confiança. Per actualitzar, copia el binari nou i torna a executar l'assistent,
triant la mateixa connexió; no esborra la configuració ni les dades.

En mode de xarxa local, el servidor anuncia el servei amb Avahi i el mòbil
pot cercar-lo a la mateixa xarxa. La descoberta pot no funcionar entre xarxes
Wi-Fi aïllades; sempre es pot escriure la IP i el port. Per internet, escriu
l'adreça HTTPS del túnel o del domini: no se n'assigna cap automàticament.
Perquè una persona nova es registri des d'Android, l'administrador genera un
codi d'un sol ús des del panell o amb `sudo gregalctl invite` i l'hi facilita
per un canal privat. El codi caduca al cap de 30 minuts; la persona tria el seu
nom i contrasenya. L'administrador també pot crear comptes directament.

Un cop iniciada la sessió d'administrador a la web, obre **Configuració →
Avançat → Comptes del servidor**. Introdueix un nom i una contrasenya de com a
mínim 12 caràcters per crear cada compte. Els comptes creats aquí no són
administradors. Cada persona pot entrar amb el seu nom i contrasenya des de la
web o des d'Android. L'administrador inicial també pot utilitzar la clau general,
que dona accés administratiu i s'ha de mantenir privada.

Les carpetes dels comptes gestionats viuen dins de
`/var/lib/gregal/projects/accounts` i es mantenen entre actualitzacions.
La gestió gràfica permet llistar i crear comptes i canviar la contrasenya
dels comptes creats aquí; en canviar-la, les sessions anteriors deixen de
ser vàlides. La baixa encara requereix administració manual. Per ara usa
comptes només amb persones de confiança: no és un sistema d'aïllament fort
entre usuaris que executin codi al mateix servidor.

## A casa (mateixa wifi)

A l'ordinador:

```bash
gregal --serve --addr 0.0.0.0
```

Surt un missatge com aquest:

```
Gregal està en marxa.
A l'app del mòbil, «Connecta't a un servidor», escriu:
  Adreça:    192.168.1.62
  Port:      8097
  Clau:      7buvc-9brxu-mujdw-emenx
```

Al mòbil: toca **Afegeix un servidor** (a l'interruptor de dalt), escriu les
tres coses i **Connecta**. La clau es crea la primera vegada i es desa: és la
mateixa a cada arrencada.

Fora de casa, el punt de l'interruptor es posa vermell i continues al mòbil.

## A internet (una màquina sempre encesa amb el teu domini)

Per arribar-hi des de qualsevol lloc cal un domini amb HTTPS. Necessites:

- Una màquina Linux amb systemd (Debian o Ubuntu per a la instal·lació
  automàtica de Caddy): un VPS o un mini-PC.
- Un domini o subdomini que hi apunti (registre A, i AAAA si té IPv6).
- Els ports 80 i 443 oberts (el 80 només el fa servir Let's Encrypt).

1. Compila el binari per a Linux (o baixa'l de la release):

   ```bash
   ./build.sh
   ```

   Surt a `dist/gregal-linux-amd64` (o `-arm64` per a una Raspberry Pi).

2. Copia el binari i l'instal·lador a la màquina:

   ```bash
   scp dist/gregal-linux-amd64 deploy/linux/install.sh deploy/linux/setup.sh deploy/linux/gregalctl.sh deploy/linux/backup.sh usuari@servidor:
   ```

3. A la màquina, executa l'instal·lador amb el teu domini:

   ```bash
   sudo sh install.sh --domain vents.exemple.cat --binary ./gregal-linux-amd64 --email tu@exemple.cat
   ```

   En acabar diu què escriure a l'app:

   ```
   Gregal ja corre. A l'app del mòbil, «Connecta't a un servidor», escriu:
     Adreça:  https://vents.exemple.cat
     Port:    (no cal)
     Clau:    xxxxx-xxxxx-xxxxx-xxxxx
   ```

4. Obre `https://vents.exemple.cat` al navegador, posa-hi la clau i, a
   Configuració, la clau del proveïdor de models i la d'Exa (cerca web). Es
   desen al servidor.

### Què fa l'instal·lador

- Crea l'usuari de sistema `gregal` (casa `/var/lib/gregal`): Gregal no corre
  mai com a root i, amb l'aïllament de systemd, no pot escriure fora de la seva
  casa.
- Instal·la el servei `gregal`, que escolta només a `127.0.0.1:8097` i es
  reinicia sol si cau o si la màquina es reinicia.
- Genera la clau d'accés i la desa a `/etc/gregal/env` (només root): no surt a
  la llista de processos ni al config. Es conserva si el tornes a executar.
- Posa Caddy al davant: certificat HTTPS automàtic, HSTS i les respostes en
  directe dels vents sense memòria intermèdia.
- Opcional: `--user NOM` afegeix també un compte amb contrasenya (per a
  servidors amb més d'una persona).

Es pot tornar a executar per actualitzar el binari: no toca la clau, el config,
els comptes ni les dades.

## Seguretat

- La clau té 20 caràcters aleatoris (uns 99 bits): no es pot endevinar.
- Els intents d'entrada amb contrasenya fallits es limiten per adreça (8 cada 5
  minuts) i en total (60 cada 5 minuts). L'adreça es llegeix de la connexió;
  `X-Forwarded-For` només es creu si ve del proxy de la mateixa màquina.
- Capçaleres `X-Content-Type-Options`, `X-Frame-Options` i `Referrer-Policy` a
  totes les respostes; HSTS des de Caddy.
- Límits de temps per a les capçaleres i les connexions inactives.
- Per canviar la clau, esborra la línia `GREGAL_API_TOKEN` de `/etc/gregal/env`,
  torna a executar l'instal·lador i reconnecta el mòbil.

## Feines llargues

Les feines dels equips de vents corren al servidor: tancar l'app o el navegador
no les atura. Al mòbil, la targeta **Feines** de la pantalla dels vents diu les
que són en marxa, qui hi treballa i els fitxers que en surten (una web s'obre
com a web); quan una acaba, arriba una notificació encara que l'app estigui
tancada, i tocar-la obre el fil de la feina. El pas en què són es desa al disc; si el servidor es reinicia a
mitja feina, l'app ho diu («aturada: el servidor es va reiniciar») en lloc de
quedar-se esperant.

## Manteniment

- Registre (inclou l'adreça, el port i la clau en arrencar):
  `journalctl -u gregal -f`
- Reiniciar: `sudo systemctl restart gregal`
- Actualitzar: torna a copiar el binari nou i executa `install.sh` amb les
  mateixes opcions.
- Dades: tot és a `/var/lib/gregal` (fes-ne còpia de seguretat).

### Còpies de seguretat i canvi de versió

Les dades viuen al servidor, no al binari ni al mòbil connectat: configuració,
comptes i vents a `/var/lib/gregal/.config/gregal`, converses i altres dades
a `/var/lib/gregal/.local/share/gregal`, i projectes a `/var/lib/gregal/projects`.
El mateix compte veu el seu estat des de qualsevol dispositiu connectat al
mateix servidor. Els proveïdors i algunes preferències són compartits pel
servidor; no tots els paràmetres són personals. El mode «Aquest dispositiu»
del mòbil té dades independents i no se sincronitza automàticament.

Abans d'una actualització i regularment, fes una còpia privada coherent:

```sh
sudo systemctl stop gregal
sudo sh backup.sh /var/backups/gregal
sudo systemctl start gregal
```

`backup.sh` és a `deploy/linux/backup.sh`: copia tota la casa del servei i
`/etc/gregal/env`, comprova que l'arxiu es pugui llegir i no funciona amb el
servei actiu. Si falla, torna a engegar el servei manualment. La còpia conté
converses, claus i contrasenyes xifrades: protegeix-la i guarda'n una altra en
una màquina diferent. No inclou la configuració de Caddy ni de cloudflared.
Per restaurar-la, atura el servei, conserva la instal·lació actual en una altra
còpia i extreu l'arxiu a una ubicació temporal per revisar-lo abans de substituir
res. Restaura tant `/var/lib/gregal` com `/etc/gregal/env` i els permisos del
servei; no barregis dades de comptes sense verificar-ne el propietari.

Per accedir-hi per cloudflared, apunta el túnel al servei local
`http://127.0.0.1:8097`; mantén l'autenticació de Gregal i no exposis el port
backend directament a internet. Per posar en marxa el servei sense Caddy,
utilitza l'instal·lador amb `--no-caddy`. El compte i la carpeta de dades han
de ser els mateixos a cada actualització. La clau d'aparellament i un compte
amb nom poden veure espais diferents: entra sempre amb el mateix compte.
