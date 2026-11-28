# Connectar un compte de Google

Gregal pot llegir el calendari, el correu i el Drive d'un compte de Google, i
hi pot preparar esdeveniments i esborranys de correu (mai els envia: els
esborranys els revises i envies tu des de Gmail). Cada persona del servidor
connecta el seu compte des de **Preferències › Connexions**; els tokens es
guarden xifrats al Vault de connexions, mai al `config.yaml`.

Google no deixa que una app es registri sola, així que qui instal·la Gregal ha
de crear un client OAuth un cop. Són uns 10 minuts.

## 1. Crear el projecte i activar les API

1. Ves a <https://console.cloud.google.com/> amb el compte que vulguis fer
   servir com a propietari i crea un projecte (per exemple «Gregal»).
2. A **APIs & Services › Library**, activa:
   - **Google Calendar API**
   - **Gmail API**
   - **Google Drive API**

## 2. Pantalla de consentiment (mode de prova)

1. **APIs & Services › OAuth consent screen** (o **Google Auth Platform ›
   Branding**): tipus **External**, nom de l'app «Gregal», el teu correu com a
   contacte.
2. **Audience**: deixa-la en **Testing** i afegeix com a **Test users** els
   comptes que connectaràs (fins a 100). En mode de prova no cal la
   verificació de Google ni l'avaluació de seguretat CASA que demanen Gmail i
   Drive a les apps públiques.
3. **Data access / Scopes**: afegeix-hi els mateixos àmbits que demana Gregal:
   - `openid`, `.../auth/userinfo.email`, `.../auth/userinfo.profile`
   - `https://www.googleapis.com/auth/calendar.events`
   - `https://www.googleapis.com/auth/gmail.readonly`
   - `https://www.googleapis.com/auth/gmail.compose`
   - `https://www.googleapis.com/auth/drive.readonly`

En mode de prova Google mostra l'avís «Google no ha verificat aquesta app» en
connectar: és normal, tria **Continua**. Els tokens de refresc d'una app en
prova caduquen al cap d'uns dies; quan passi, Gregal marcarà la connexió com
«cal tornar a iniciar» i n'hi ha prou de tornar a connectar.

## 3. Crear el client OAuth

1. **APIs & Services › Credentials › Create credentials › OAuth client ID**.
2. Tipus **Web application**.
3. **Authorized redirect URIs**: l'adreça del teu Gregal seguida de
   `/api/mcp/oauth/callback`. Per exemple:
   - app d'escriptori o web local: `http://127.0.0.1:8097/api/mcp/oauth/callback`
   - servidor amb domini: `https://gregal.example.org/api/mcp/oauth/callback`

   Ha de coincidir exactament (esquema, host, port) amb l'adreça des d'on
   obres Gregal quan connectes el compte. Si l'obres de dues maneres, posa-hi
   les dues.
4. Copia el **Client ID** i el **Client secret**.

## 4. Posar-ho al config

Al `config.yaml` de Gregal (o a Preferències si l'edites des de la web):

```yaml
google:
  client_id: 1234567890-abc.apps.googleusercontent.com
  client_secret: ${GREGAL_GOOGLE_CLIENT_SECRET}
```

El secret pot anar escrit o com a `${VARIABLE}` d'entorn (recomanat). Reinicia
Gregal i comprova-ho amb `gregal --check-config`.

## 5. Connectar

**Preferències › Connexions › Google › Connecta**: s'obre el consentiment de
Google, tries el compte i els permisos, i tornes a Gregal. Pots connectar més
d'un compte. Des d'aquí també pots:

- **Pausar la lectura** o **permetre accions** (crear esdeveniments i
  esborranys). Per defecte només llegeix.
- **Desconnectar**: Gregal revoca el permís a Google i esborra el token.

## Què hi poden fer els vents

Amb l'eina `connector` sobre la connexió de Google:

| Acció | Tipus | Per a què |
| --- | --- | --- |
| `calendar_events` | lectura | agenda d'uns dies, preparar reunions, resum del matí |
| `gmail_search` | lectura | safata d'entrada, correus d'una persona, pendents de resposta |
| `gmail_read` | lectura | llegir un correu sencer |
| `drive_search` | lectura | trobar documents i fulls |
| `calendar_create` | acció | crear un esdeveniment (cal permetre accions) |
| `gmail_draft` | acció | preparar una resposta perquè l'enviïs tu |

Exemples: «Cada dia feiner a les 8, resumeix-me l'agenda i els correus no
llegits», «Prepara'm la reunió de les 11: qui ve i què ens hem escrit», «Fes
un esborrany de resposta a l'Anna dient que dijous em va bé».

Tot el que arriba de Google es tracta com a contingut no fiable: els vents no
segueixen instruccions escrites dins d'un correu o un document.
