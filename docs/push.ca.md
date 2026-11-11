# Avisos al mòbil: push opcional (B7)

Gregal funciona sense cap push. Per defecte l'app rep les novetats amb la
connexió de primer pla (stream d'esdeveniments amb cursor) quan és oberta i
amb una comprovació periòdica (cada 15 min, també en Doze) quan és tancada.
`gregal doctor` i Preferències → Servidor diuen «push no configurat».

El push (Firebase Cloud Messaging) només fa que l'avís arribi abans amb l'app
tancada. El missatge només porta un identificador opac; l'app llegeix el
contingut amb la seva sessió. No hi ha cap projecte de Firebase, ID de paquet
ni clau del projecte Gregal al codi: cada instal·lació o fork fa servir el seu.

## Servidor: un sol valor

| Variable | Valor | Per defecte |
|---|---|---|
| `GREGAL_FCM_CREDENTIALS` | Camí al JSON del compte de servei del teu projecte de Firebase (rol «Firebase Cloud Messaging API Admin») | Buit: sense push, mode `polling` |

El fitxer viu fora del repositori; el contingut no surt mai per cap API ni log.
Si el camí és dolent, `gregal doctor` ho marca com a error i el servidor
continua sense push. Els dispositius es registren sols (`/api/v2/push`).

## App Android (forks): una propietat de Gradle opcional

```
gregal.fcm.config=/camí/al/google-services.json
```

A `~/.gradle/gradle.properties` o amb `-P`. Sense ella l'APK no porta
Firebase i el servei de push queda desactivat al manifest. Amb ella, el
`google-services.json` ha de tenir un client Android per a l'ID de paquet de
la build (`xyz.solanes.gregal`, o `xyz.solanes.gregal.vents` amb
`-Pgregal.plotsPreview`).

Cal tots dos costats (servidor i app) del mateix projecte de Firebase perquè
Preferències digui «Push actiu».
