---
nom: sessions
descripcio: on es desen les converses, com llistar-les, reprendre-les, esborrar-les o buscar-hi dins
---

Cada conversa es desa en un JSON. No hi ha base de dades: són fitxers, i
els pots llegir i esborrar amb les eines normals.

## On són

`~/.local/share/gregal/sessions/` (Windows:
`C:/Users/<tu>/.local/share/gregal/sessions/`), o dins de
`$GREGAL_DATA_DIR/sessions` si la variable hi és.

Cada fitxer es diu `sessio-AAAAMMDD-HHMMSS-mmm.json`. Al mateix
directori hi ha `events.jsonl`, que no és cap sessió.

## Què hi ha a dins

```json
{
  "name": "sessio-20260921-145033-171",
  "title": "primer missatge de la conversa, retallat",
  "saved_at": "2026-09-21T14:50:33Z",
  "role": "chat",
  "workspace": "/ruta/del/projecte",
  "pinned": false,
  "compacted": "resum de la part compactada, si n'hi ha",
  "convo": [{"role": "user", "content": "..."}],
  "activity": [{"name": "bash", "args": "...", "output": "...", "failed": false}]
}
```

`convo` són els missatges de la persona i les teves respostes. Les
crides d'eina i les seves sortides senceres NO hi són: el que en queda
és el resum d'`activity`. Per això el comptador de missatges d'una
sessió llarga és més baix del que sembla mirant la pantalla.

La conversa es desa en acabar cada torn, i també en sortir amb `/quit` o
Ctrl+C. Si el procés mor de cop, es conserva fins a l'últim torn acabat.

## Ordres del TUI

- `/sessions` — selector amb fletxes, amb l'última conversa ja triada.
- `/resume <nom o N>` — reprèn una conversa.
- `/save [nom]` — desa la d'ara amb un nom.
- `/pin` — fixa o desfixa la conversa actual.
- `/clear` — neteja la conversa de la pantalla.

## El que NO té ordre pròpia

Esborrar, reanomenar o buscar dins de les converses. Fes-ho amb les
eines, que per això són fitxers:

```bash
# les deu més recents, amb data i mida
ls -lt ~/.local/share/gregal/sessions/sessio-*.json | head

# què hi diu una
cat ~/.local/share/gregal/sessions/sessio-20260921-145033-171.json

# buscar-hi dins (títol i contingut)
grep -l "plataformes" ~/.local/share/gregal/sessions/*.json

# esborrar-ne una de concreta
rm ~/.local/share/gregal/sessions/sessio-20260918-191945-786.json

# esborrar les de fa més de trenta dies
find ~/.local/share/gregal/sessions -name 'sessio-*.json' -mtime +30 -delete
```

Abans d'esborrar res: ensenya què esborraràs i espera que t'ho confirmin.
Una conversa esborrada no es recupera, i les que tenen `"pinned": true`
són les que algú ha volgut guardar expressament.
