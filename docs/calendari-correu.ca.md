# Google Calendar, Gmail i el resum del matí

Els vents poden mirar el teu calendari i el teu correu i fer-te un resum cada
matí. Cada compte té els seus: tu connectes el teu i la Carla el seu.

**Només lectura.** Els vents no envien, no responen, no accepten invitacions
ni esborren res. La bústia s'obre en mode només lectura i llegir un correu no
el marca com a llegit. Si cal respondre, el vent t'escriu l'esborrany i
l'envies tu.

## Connectar-ho

A **Preferències → Calendari i correu** (web, escriptori o mòbil).

### Google Calendar

1. Obre [la configuració de Google Calendar](https://calendar.google.com/calendar/r/settings).
2. A l'esquerra, tria el teu calendari i ves a **Integra el calendari**.
3. Copia l'**Adreça secreta en format iCal** (acaba en `basic.ics`).
4. Enganxa-la a Gregal amb un nom (per exemple «Personal») i toca **Afegeix**.

Pots afegir-ne fins a 10: la feina, la família, els festius. L'adreça secreta
és una clau, així que Gregal no la torna a mostrar sencera. Si algun dia la
comparteixes per error, a Google pots generar-ne una de nova («Restableix»).

L'hora sempre és la del teu calendari, encara que el servidor vagi en UTC.

### Gmail

1. Cal tenir la [verificació en dos passos](https://myaccount.google.com/signinoptions/two-step-verification)
   activada.
2. Crea una [contrasenya d'aplicació](https://myaccount.google.com/apppasswords)
   (per exemple «Gregal»). Google la mostra en quatre grups de quatre lletres.
3. A Gregal escriu el teu Gmail i aquesta contrasenya, i toca **Connecta**.
   Gregal ho prova en el moment. Si diu que Gmail l'ha rebutjat, normalment és
   que s'ha fet servir la contrasenya normal en lloc de la d'aplicació.

Amb comptes de Google Workspace (de feina), l'administrador del domini ha de
permetre les contrasenyes d'aplicació i l'IMAP.

## Què hi poden fer els vents

- **Calendari:** què tens avui, demà, aquesta setmana o entre dues dates, amb
  els solapaments marcats.
- **Correu:**
  - els que no has llegit i els dels últims dies;
  - cerques amb la mateixa sintaxi de Gmail (`from:anna`,
    `subject:factura`, `newer_than:7d`, `is:unread`);
  - llegir-ne un.

Ho fan sols quan els preguntes («què tinc demà?», «m'ha escrit la gestoria?»).
El text dels correus el tracten com a informació, mai com a ordres.

## Resum del matí

A la mateixa targeta: activa **Resum del matí**, tria l'hora i quin vent te'l
fa (el «Gregal» de la colla del dia a dia és el d'organització). Cada dia, a
aquella hora, et diu en un missatge curt:
- què tens al calendari i a quina hora comences;
- quins correus demanen atenció;
- què tens pendent.

Et surt com a notificació al mòbil. Quan l'actives no s'envia de seguida: el
primer arriba a la propera hora triada.
