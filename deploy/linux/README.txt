GREGAL — SERVIDOR LINUX (Intel/AMD 64 bits)

1. Descomprimeix el paquet i entra en aquesta carpeta.
2. Executa: sudo sh setup.sh
   Per a un agent, sense preguntes: sudo -n sh setup.sh --mode tunnel --admin aleix
3. Tria el tipus de connexió i segueix les preguntes.
4. Desa la contrasenya de l'administrador que es mostra en acabar.
5. A la web, ves a Configuració > Avançat > Comptes del servidor.
   Des del terminal (contrasenyes llegides per stdin, mai en arguments):

     sudo -n gregalctl accounts
     sudo -n gregalctl invite
     printf '%s\n' "$CONTRASENYA" | sudo -n gregalctl account-create anna
     printf '%s\n' "$CONTRASENYA_NOVA" | sudo -n gregalctl account-password anna
     sudo -n gregalctl status
     sudo -n gregalctl backup

   No enganxis contrasenyes literals a les ordres ni a l'historial. Les variables
   anteriors han de provenir d'un canal segur i no es defineixen en aquest fitxer.
   account-password només modifica comptes creats des del panell o gregalctl.
   sudo -n requereix que sudo ja estigui autoritzat sense pregunta.

En mode LAN, Android cerca servidors a la mateixa xarxa. Per internet s'ha
d'escriure la URL HTTPS. Cada invitació caduca en 30 minuts i només serveix una vegada.

Cloudflare Tunnel s'ha de configurar a part apuntant a http://127.0.0.1:8097.
L'opció de xarxa local fa servir HTTP sense xifrar; usa-la només en una xarxa de confiança.

Per actualitzar, fes una còpia de seguretat i torna a executar setup.sh amb el
binari nou i el mateix tipus de connexió. Les dades es conserven a /var/lib/gregal.
La còpia inclou dades personals i claus: protegeix-la i guarda-la fora del servidor.

  sudo gregalctl backup

Aquest paquet és una versió de desenvolupament, no una release publicada.
