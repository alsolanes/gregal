---
nom: configuracio
descripcio: el config.yaml del gregal (rols, proveïdors, permisos, verificació, pressupost de passos) i on van les claus d'API
---

## Els dos fitxers

- `~/.config/gregal/config.yaml` — tot menys els secrets. Es pot
  ensenyar i versionar.
- `~/.config/gregal/auth.yaml` — les claus d'API, a part. No l'obris per
  ensenyar-ne el contingut i no copiïs mai una clau a la conversa.

`gregal --config <ruta>` en fa servir un altre. `gregal --check-config`
el valida i surt; `gregal --doctor` mira també si els proveïdors
responen.

## Les claus que hi ha

- `providers:` — un per endpoint, amb `base_url` i `api_key`. L'api_key
  admet `${VARIABLE}` per llegir-la de l'entorn.
- `roles:` — quina feina fa quin model. Els rols que han d'existir són
  `chat`, `think`, `code` i `reviewer`; cadascun té `provider`, `model`,
  `temperature`, `max_tokens` i, opcionalment, `context_window` (0 = la
  que declari el proveïdor).
- `mode:` — el mode d'arrencada (`code`, `chat`, `inspect`, `goal`,
  `autonomous`). Vegeu la skill `modes`.
- `permissions:` — `tools:` amb `allow`, `ask` o `deny` per eina, més
  `bash_allow` i `bash_deny` per acotar la shell. Les eines que no hi
  són segueixen el criteri per defecte: llegir va sol, escriure i la
  shell pregunten.
- `verify:` — `mode: off|manual|auto|both|strict`: si un revisor repassa
  la feina en acabar el torn.
- `agent.max_steps` — el pressupost de passos d'un torn. Quan s'esgota,
  es pregunta si val la pena continuar. Els torns que es queden a mitges
  amb «s'han acabat els N passos» normalment volen dir que aquest nombre
  és massa baix per a la feina que es demana.
- `agent.autonomous` — `checkpoint_every`, `review_every`, `max_minutes`
  del mode autònom.
- `router:` — tria automàtica de rol segons la tasca (`mode: auto|off`).
- `hooks:` — `post_edit` corre una ordre amb `{file}` després de cada
  escriptura amb èxit (per exemple `gofmt -w {file}`), i `diag` tria si
  es fa comprovació de sintaxi.
- `system:` — el prompt d'identitat. Buit = el del gregal.
- `theme:` (`fosc`|`clar`), `lang:` (`ca`|`en`), `mouse:` (`on`|`off`),
  `animacions:` (`on`|`off`, per defecte quietes) i `cockpit:`
  (`on`|`off`, la columna de la dreta del TUI).
- `mcp:` — servidors MCP; les seves eines surten amb el prefix `mcp_`.
- `telegram:` — token i usuaris autoritzats del bot.
- `cost:` — preus per milió de tokens, per calcular el cost de la sessió.

## Canviar-ho sense editar el YAML

Al TUI, `/settings` (o Ctrl+,) ho concentra tot: mode, rol, model, clau
d'API, tema, revisor, permisos i proveïdors. També hi ha `/permissions`,
`/model`, `/role`, `/reviewer`, `/provider`, `/key`, `/theme` i `/mouse`.
Tot el que es tria des d'allà es desa al config.

Si t'han demanat un canvi de configuració, la manera neta és dir-los
quina ordre del TUI ho fa. Editar el YAML amb `edit` també val, però
llavors cal reiniciar el gregal perquè el llegeixi.
