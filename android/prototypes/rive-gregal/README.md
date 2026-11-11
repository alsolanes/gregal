# Retrat de Gregal amb Rive

Prototip vectorial de 240 × 240 per a la portada dels Vents. La màquina d'estats
`Retrat` combina l'activitat del vent amb una reacció al toc. Les dues capes
permeten prémer l'avatar en qualsevol estat.

El model `Gregal` rep el número `activity` des de l'aplicació.

| Valor | Estat |
| --- | --- |
| 0 | Repòs, amb un parpelleig periòdic |
| 1 | Lectura, amb un llibre i un moviment de la mirada |
| 2 | Feina enllestida, amb una animació breu i una marca persistent |

El retrat gestiona `pressed` en prémer, deixar anar o sortir de la seva àrea
d'interacció.

Des de l'arrel del repositori, amb la CLI de Rive disponible:

```powershell
rive android/prototypes/rive-gregal --verify
rive android/prototypes/rive-gregal
rive android/prototypes/rive-gregal --android
```

Es poden comprovar els estats i el toc en mode sense finestra:

```powershell
rive android/prototypes/rive-gregal --data=activity=1 --advance=500ms --screenshot
rive android/prototypes/rive-gregal --data=activity=2 --advance=700ms --screenshot
rive android/prototypes/rive-gregal '--pointer=down@120,116' --advance=150ms --data-dump=-
```

L'exportació del recurs per a l'APK requereix una sessió de Rive. Un pla de
pagament permet exportar el recurs sense la pantalla de marca.

```powershell
rive login
rive android/prototypes/rive-gregal --publish
```
