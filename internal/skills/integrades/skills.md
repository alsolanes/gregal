---
nom: skills
descripcio: com funcionen les skills i com se n'escriu una de nova per a un projecte o per a tu
---

Una skill és un document que només entra al teu context quan el demanes.
Al prompt hi tens l'índex (nom i una línia); el cos arriba quan crides
l'eina `skill` amb el nom.

Serveix per al coneixement que és massa llarg per portar sempre i massa
concret per deduir-lo: com es desplega aquest projecte, quines
convencions té l'equip, com es fa una operació que no té ordre pròpia.

## D'on surten

1. Les integrades, dins del binari: expliquen el gregal a si mateix.
2. `~/.config/gregal/skills/*.md` — les d'aquesta persona, a totes les
   feines.
3. `.gregal/skills/*.md` a l'arrel del projecte — les d'aquest
   projecte, i van amb el repositori.

Si dues es diuen igual, mana la de més avall de la llista: un projecte
pot corregir el que el gregal creu saber.

## El format

```markdown
---
nom: desplegament
descripcio: com es desplega aquest servei a producció i qui ho ha d'aprovar
---

El cos, en Markdown. Tan llarg com calgui: només es llegeix quan algú
el demana.
```

Les dues claus de la capçalera són obligatòries. Un fitxer sense
capçalera s'ignora, perquè no posem qualsevol README a l'índex del
prompt.

La descripció és el que decideix si la skill s'obre o no: ha de dir en
quins casos serveix, no què conté. «com es desplega a producció i qui ho
ha d'aprovar» és útil; «notes de desplegament» no.

## Escriure'n una

Quan et demanin que recordis com es fa alguna cosa d'aquest projecte,
proposa una skill: `.gregal/skills/<nom>.md`, amb la capçalera i el que
s'ha après. La pots crear amb `write` com qualsevol altre fitxer. La
propera sessió ja la tindrà a l'índex.

Per a un fet curt (una preferència, una decisió) no cal una skill:
`/nota` ho desa a `.gregal/memory.md` i això sí que entra sempre al
prompt.
