# Servidor propi a internet

Gregal pot córrer en una màquina Linux sempre encesa (un VPS o un mini-PC a
casa) i obrir-se al teu domini amb HTTPS. Així els vents poden fer feines
llargues (webs, anàlisis, equips sencers) que continuen encara que tanquis
l'app, i les consultes des del mòbil o des de qualsevol navegador.

## Què cal

- Una màquina Linux amb systemd (Debian o Ubuntu per a la instal·lació
  automàtica de Caddy).
- Un domini (o subdomini) que apunti a aquesta màquina: un registre A (i AAAA
  si té IPv6).
- Els ports 80 i 443 oberts a internet (el 80 només el fa servir Let's
  Encrypt per emetre el certificat).

## Instal·lació

1. Compila el binari per a Linux des del repositori (o baixa'l de la release):

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
   sudo sh install.sh --domain vents.exemple.cat --binary ./gregal-linux-amd64 --user aleix --email tu@exemple.cat
   ```

   Al final mostra l'adreça, l'usuari i una contrasenya generada. Desa-la:
   no es torna a mostrar.

4. Obre `https://vents.exemple.cat` al navegador, entra i, a Configuració,
   posa la clau del proveïdor de models i la d'Exa (cerca web). Es desen al
   servidor.

5. Al mòbil: «Canvia de servidor» → «O connecta't a un servidor…», escriu
   `https://vents.exemple.cat` (el port no cal), l'usuari i la contrasenya.

## Què fa l'instal·lador

- Crea l'usuari de sistema `gregal` (casa `/var/lib/gregal`): Gregal no
  corre mai com a root i, amb l'aïllament de systemd, no pot escriure fora de
  la seva casa.
- Instal·la el servei `gregal`, que escolta només a `127.0.0.1:8097` i es
  reinicia sol si cau o si la màquina es reinicia.
- Afegeix un compte d'administrador amb contrasenya (xifrada amb PBKDF2): a
  internet cadascú entra amb el seu nom, no hi ha cap token compartit.
- Posa Caddy al davant: certificat HTTPS automàtic, HSTS i els fluxos en
  directe dels vents sense memòria intermèdia.

Es pot tornar a executar per actualitzar el binari: no toca el config, els
comptes ni les dades.

## Seguretat

- Els intents d'entrada fallits es limiten per adreça (8 cada 5 minuts) i en
  total (60 cada 5 minuts). L'adreça es llegeix de la connexió; la capçalera
  `X-Forwarded-For` només es creu si ve del proxy de la mateixa màquina.
- Capçaleres `X-Content-Type-Options`, `X-Frame-Options` i `Referrer-Policy`
  a totes les respostes; HSTS des de Caddy.
- Límits de temps per a les capçaleres i les connexions inactives.
- Per canviar la contrasenya:

  ```bash
  sudo runuser -u gregal -- env HOME=/var/lib/gregal gregal --config /var/lib/gregal/.config/gregal/config.yaml --passwd aleix
  ```

- Per afegir una persona, afegeix-la a la secció `users:` de
  `/var/lib/gregal/.config/gregal/config.yaml` i dona-li contrasenya amb la
  mateixa ordre.

## Feines llargues

Les feines dels equips de vents corren al servidor: tancar l'app o el
navegador no les atura. El pas en què són es desa al disc; si el servidor es
reinicia a mitja feina, l'app ho diu («aturada: el servidor es va
reiniciar») en lloc de quedar-se esperant.

## Manteniment

- Registre: `journalctl -u gregal -f`
- Reiniciar: `sudo systemctl restart gregal`
- Actualitzar: torna a copiar el binari nou i executa `install.sh` amb les
  mateixes opcions.
- Dades: tot és a `/var/lib/gregal` (fes-ne còpia de seguretat).
