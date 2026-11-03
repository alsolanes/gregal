# Vents com a assistents: què fan Dot, dots i Grok, i com portar-ho a Gregal

Proposta del 2 de novembre de 2026. L'objectiu és tenir petits assistents d'IA
(els vents) que se n'encarreguin de qualsevol tasca, que comprovin que queda bé
i funciona, i que parlin quan cal. També han de ser fàcils d'estendre: models
de Claude o Codex amb la subscripció, calendari, Gmail i plugins.

## 1. Què fan els referents

| | Dot (New Computer) | dots (OpenAI) | Grok (xAI) |
| --- | --- | --- | --- |
| Estat | Tancat l'octubre del 2025 | Nou (DevDay, 29/9/2026), Pro o superior, fora d'Europa | Tasks i Automations (juliol 2026) |
| Idea | Company que et coneix | Agent sempre encès amb ordinador i navegador propis | Feines programades i per esdeveniment |
| Memòria | Memòria «agèntica» amb estat i data (pendent, previst); mapa del que sap | El que sap de tu, revisat periòdicament | Per conversa |
| Proactivitat | Et prepara per a esdeveniments; treu el que és rellevant sense que ho demanis | Revisa periòdicament què sap de tu i busca maneres d'ajudar; avisa de conflictes al calendari i de terminis | Programa (diari, setmanal…) o un correu entrant dispara la feina; resultat per notificació o correu |
| Integracions | Notes, fitxers, àudio | Més de 4.000 apps, calendari, correu, Codex, Slack/Teams | Connectors amb @, correu |
| Seguiment | — | Fa ell mateix el seguiment d'una feina en curs (follow-ups) | Historial d'execucions; «Executa ara» per provar |
| Límits | — | Les accions sensibles (contrasenyes) sempre les fas tu | Només el correu dispara feines |

**Idees que valen la pena:**
1. Una feina es descriu en llenguatge natural i es converteix en una automatització (Grok: «cada matí mira…»).
2. Disparadors per esdeveniment, no només per hora (un correu nou, un esdeveniment que s'acosta).
3. Proactivitat amb criteri: revisar periòdicament què sap de tu i proposar ajuda (dots), però callar si no hi ha res (Gregal ja ho fa amb «(res)»).
4. Seguiment: l'assistent torna sol a una feina inacabada fins que funciona.
5. Les accions sensibles sempre passen per tu: enviar, pagar, esborrar, contrasenyes.

## 2. Què ja té Gregal (no cal tornar-ho a fer)

- **Vents amb memòria i personalitat:**
  - memòria per vent i cervell;
  - lliçons i habilitats;
  - fitxes de referència;
  - notes del servidor compartides.
- **Feina autònoma:**
  - feines (`/api/winds/tasks`) i colles d'especialistes;
  - poders de servidor;
  - `app`, `publish` i `tunnel`.
- **Proactivitat:**
  - batecs per vent: vigila el seu tema, parla només si val la pena, i si no respon «(res)» i es programa la propera mirada;
  - recordatoris (`remind`);
  - rutines diàries o per interval que obren un torn de veritat;
  - novetats i notificacions al mòbil, que arriben a l'hora.
- **Connectors MCP:**
  - servidors per stdio o per HTTP;
  - OAuth 2.1 complet (descoberta, registre dinàmic, PKCE, refresc);
  - marca de només lectura;
  - importació de la configuració de Claude Code, Codex i opencode.
- **Models:** Anthropic (API Messages), OpenAI (xat i Responses), Azure i Foundry, OpenRouter i compatibles, Ollama, llama.cpp.
- **Plugins:** Racó, Jocs i Arena. Es compilen dins del binari; poden afegir rutes, un bucle de fons i una vista, però no eines.

**Què falta:**
- calendari i correu;
- un resum del matí;
- disparadors per esdeveniment;
- seguiment automàtic de feines inacabades;
- plugins de tercers carregats en calent;
- models a través d'una subscripció (Claude Max, ChatGPT).

## 3. Proposta, per prioritat

### P1 · Models de Claude i Codex amb la teva subscripció

**Avui, sense codi:**
- Amb una clau d'API d'Anthropic o d'OpenAI, a Preferències → proveïdor.
- Els plans Max i Team de Claude ja inclouen crèdits d'API cada mes (100 $ amb Max 5x, 200 $ amb Max 20x). S'han de reclamar a la Console i fer-los servir amb una clau d'API. És el camí que Anthropic recomana per a eines de tercers.

**Nou: «motor extern» (`engine: claude-code` / `engine: codex`).**
- En lloc de cridar l'API del model, Gregal passa el torn sencer a la CLI oficial que ja tens instal·lada i iniciada:
  - `claude -p --output-format stream-json`;
  - `codex exec --json`, que fa servir la sessió de ChatGPT.
- Les eines de Gregal (`publish`, `app`, `tunnel`, `remind`, memòria del vent) s'hi exposen amb un servidor MCP local (`gregal mcp tools`). Així el vent continua sent el vent: personalitat, memòria i eines. Només canvia qui fa el bucle.
- **Per què així:** és la manera de fer servir els límits de la subscripció i no pagar per token. A més, el bucle de Claude Code o de Codex és molt bo en programació.
- **Condicions:**
  - Només per a ús personal, a la teva màquina, amb la teva sessió.
  - Anthropic ha dit fa pocs dies que l'Agent SDK i `claude -p` amb la subscripció Max es poden continuar fent servir. El que no permet és oferir el login de claude.ai als usuaris d'un producte.
  - Per a la Marta, cada persona hauria d'iniciar sessió a la seva pròpia CLI.
- **Feina:**
  - un nou tipus de motor a `internal/web` que tradueixi el flux d'events (tokens, eines, final) als events dels vents;
  - el servidor MCP d'eines;
  - triar el motor per vent o per a tots.
  - Uns 2–3 dies.

### P2 · Calendari i Gmail

**Ràpid (lectura, sense Google Cloud):**
- **Calendari:** l'URL privada iCal de Google Calendar (o de qualsevol calendari). Gregal la llegeix cada pocs minuts, i els vents la tenen com a eina `calendar` de només lectura (avui, setmana, conflictes).
- **Gmail:** IMAP amb una contrasenya d'aplicació. Eina `mail` de només lectura: llegir, cercar i comptar els no llegits.
- Mig dia cadascun.

**Complet (llegir i escriure):**
- Els servidors MCP remots oficials de Google: `gmailmcp.googleapis.com` i `calendarmcp.googleapis.com`. Gregal ja té l'OAuth de MCP, només cal afegir-hi els preajusts. Permeten llegir correus, crear esborranys, posar etiquetes, i crear, moure i respondre esdeveniments.
- Encara són en *developer preview*. Cal un projecte de Google Cloud amb els serveis activats, un client OAuth i ser al programa de preview.
- **Regla de seguretat:**
  - els vents poden llegir sols;
  - enviar un correu o acceptar o crear un esdeveniment ho proposen amb una targeta «Envia / Edita / Descarta», i ho fas tu;
  - els esborranys sí que els poden crear sols.

### P3 · Proactivitat a l'estil dots

- **Resum del matí:** una rutina predefinida, a l'hora que triïs, amb:
  - el calendari del dia i els conflictes;
  - correus importants;
  - recordatoris i feines pendents;
  - el temps i les notícies del teu tema.

  Arriba com a notificació i com a missatge d'un vent (el «Gregal» de la colla, que és el d'organització). Es basa en les rutines que ja hi ha.
- **Disparadors per esdeveniment:**
  - «30 minuts abans d'una reunió, prepara'm el context»;
  - «quan arribi un correu de X, avisa'm i resumeix-lo»;
  - «quan una web meva deixi de respondre, mira què passa».

  És un `trigger` a les rutines (hora, esdeveniment de calendari, correu nou, URL caiguda) avaluat al bucle de feines.
- **Revisió setmanal:** cada vent mira la seva memòria i proposa una cosa útil, o calla. Com els batecs, amb el «(res)».
- **Hores de silenci i pressupost:** les notificacions no surten de nit, i hi ha un màxim per dia i vent.
- Uns 2 dies sobre el que ja hi ha.

### P4 · Que se n'encarreguin fins que funcioni

- **Feines amb criteri d'acabat:** en encarregar una feina, el vent escriu com comprovarà que està bé (per exemple «la web respon 200 i mostra X»). Quan creu que ha acabat, ho comprova de debò. Si falla, ho torna a intentar fins a N cops i, si no se'n surt, et diu exactament què falla.
- **Seguiment:** el vent programa sol un recordatori de feina («d'aquí a 1 hora comprovo que el túnel continua viu») i torna a la conversa amb el resultat.
- **Historial d'execucions** amb «Executa ara», com Grok, a la nova secció Automatitzacions.
- Uns 2 dies.

### P5 · Plugins de tercers

Un plugin seria una carpeta (`~/.gregal/plugins/<id>/plugin.json`) que pot portar:
- un servidor MCP (eines);
- habilitats i plantilles de vent;
- rutines predefinides (per exemple «resum del matí»);
- opcionalment, una pàgina web que s'obre dins de l'app.

Es carrega en calent i l'administrador l'activa o el desactiva com els d'ara. Així el calendari, Gmail, Notion o una botiga es poden afegir sense recompilar Gregal, igual que els plugins de Claude Code o les apps de ChatGPT. Uns 3 dies, i els plugins d'avui (Racó, Jocs, Arena) es poden quedar com estan.

### P6 · Més àgil al mòbil

- **Compartir amb un vent:** des de qualsevol app (Android → Comparteix), un enllaç, un text o una foto amb «què en fas?».
- **Respondre des de la notificació** sense obrir l'app.
- **Notes de veu:** gravar, transcriure i enviar. També a Telegram, que ara no les accepta.
- **Una giny** a la pantalla d'inici amb el resum del dia i un botó per parlar amb un vent.
- **Parlar amb els vents per Telegram**, no només amb l'agent principal: el canal que feies servir amb Hermes.

## 4. Ordre recomanat

1. **P2 ràpid** (calendari iCal i Gmail per IMAP, de lectura) **+ P3 resum del matí.** És el que més es nota cada dia, amb poca feina i sense dependre de Google Cloud.
2. **P1 motor extern** (Claude Code i Codex amb la subscripció). És la potència per programar.
3. **P4 feines amb criteri d'acabat i seguiment.** És el «se n'encarreguen fins que funciona».
4. **P6 compartir i respondre des de la notificació.**
5. **P5 plugins** i el **P2 complet** (MCP oficial de Google, quan surti de preview).

## Fonts

- Dot: [Fast Company](https://www.fastcompany.com/91142350/dot-an-ai-companion-app-designed-by-an-apple-alum-launches-in-the-app-store), [The Rundown](https://www.therundown.ai/tools/dot), [AI Engineer](https://ai.engineer/orgs/new-computer)
- OpenAI dots: [DataCamp](https://www.datacamp.com/blog/openai-dots), [MindStudio](https://www.mindstudio.ai/blog/openai-dots-personal-agents), [TechBuzz](https://www.techbuzz.ai/articles/openai-launches-dots-ai-coding-tools-become-personal-assistant)
- Grok: [Automations](https://www.usecarly.com/blog/grok-automations/), [Tasks](https://grokipedia.com/page/Grok_Tasks), [Reminders](https://trakkr.ai/article/grok-reminders)
- Claude i subscripció: [Agent SDK](https://code.claude.com/docs/en/agent-sdk/overview), [The New Stack](https://thenewstack.io/anthropic-agent-sdk-confusion/), [HN: crèdits inclosos](https://news.ycombinator.com/item?id=49997654)
- Codex: [Mode no interactiu](https://developers.openai.com/codex/noninteractive.md), [Login ChatGPT o clau](https://inventivehq.com/blog/codex-cli-chatgpt-login-vs-api-key)
- Google: [Servidors MCP de Workspace](https://developers.google.com/workspace/guides/configure-mcp-servers), [MCP de Calendar](https://developers.google.com/workspace/calendar/api/guides/configure-mcp-server), [Anunci](https://workspaceupdates.googleblog.com/2026/05/agent-tools-and-security-updates-for-workspace-developers.html)
