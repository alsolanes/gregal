# Servidor propi

L'app d'Android funciona sola, dins del mòbil. Si vols que els vents facin
feines llargues (webs, anàlisis, equips sencers) que continuïn encara que
tanquis l'app, engega Gregal en un ordinador o servidor i connecta-hi el mòbil.
A dalt de la pantalla dels vents tens l'interruptor **📱 Al mòbil · Servidor**:
un toc i canvies. Cada lloc té els seus vents i converses.

Connectar-hi el mòbil sempre és el mateix: **adreça, port i clau**. El servidor
t'ho diu en engegar-se.

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
   scp dist/gregal-linux-amd64 deploy/linux/install.sh usuari@servidor:
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
no les atura. El pas en què són es desa al disc; si el servidor es reinicia a
mitja feina, l'app ho diu («aturada: el servidor es va reiniciar») en lloc de
quedar-se esperant.

## Manteniment

- Registre (inclou l'adreça, el port i la clau en arrencar):
  `journalctl -u gregal -f`
- Reiniciar: `sudo systemctl restart gregal`
- Actualitzar: torna a copiar el binari nou i executa `install.sh` amb les
  mateixes opcions.
- Dades: tot és a `/var/lib/gregal` (fes-ne còpia de seguretat).
