// gregal-ordinador és l'agent mínim de l'ordinador personal (paquet B2):
// corre dins la VM de cada persona, diu la seua salut i executa les eines
// que li envia l'amfitrió. No és el servidor Gregal: no té comptes,
// converses, avisos ni webs.
//
//	gregal-ordinador --listen 10.66.0.2:8098 --id pc-carla --owner carla \
//	  --files /srv/ordinador/fitxers --token-file /etc/gregal-ordinador/token
//
// El navegador és el Chrome supervisat de la VM (GREGAL_BROWSER_CDP) amb el
// perfil al volum (GREGAL_ORDINADOR_PROFILE). Vegeu deploy/ordinador.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gregal/internal/ordinador"
)

func main() {
	listen := flag.String("listen", "", "adreça privada on escolta l'agent (IP:port; mai pública)")
	id := flag.String("id", "", "identificador de l'ordinador (el del registre de l'amfitrió)")
	owner := flag.String("owner", "", "compte de Gregal propietari de l'ordinador")
	files := flag.String("files", "", "carpeta de fitxers (camí absolut, al volum persistent)")
	tokenFile := flag.String("token-file", "", "fitxer amb el testimoni compartit amb l'amfitrió")
	flag.Parse()

	die := func(f string, a ...any) {
		fmt.Fprintf(os.Stderr, "gregal-ordinador: "+f+"\n", a...)
		os.Exit(1)
	}
	if *listen == "" || *tokenFile == "" {
		die("calen --listen i --token-file")
	}
	if err := ordinador.CheckPrivateAddr(*listen); err != nil {
		die("%v", err)
	}
	tok, err := os.ReadFile(*tokenFile)
	if err != nil {
		die("testimoni: %v", err)
	}
	// La guarda del navegador (tools.attachAllowed) només deixa entrar el
	// propietari al Chrome supervisat.
	os.Setenv("GREGAL_ORDINADOR_OWNER", *owner)
	h, err := ordinador.NewAgent(ordinador.AgentConfig{
		ID: *id, Owner: *owner, FilesDir: *files, Token: strings.TrimSpace(string(tok)),
	})
	if err != nil {
		die("%v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stderr, "gregal-ordinador %s (%s) escolta a %s\n", *id, *owner, *listen)
	if err := ordinador.ServeAgent(ctx, *listen, h); err != nil {
		die("%v", err)
	}
}
