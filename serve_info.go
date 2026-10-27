package main

// Missatge d'arrencada del servidor: què cal escriure a l'app del mòbil
// (adreça, port i clau) i res més. Posar Gregal en marxa per al mòbil ha de
// ser «engega'l i copia tres coses».

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gregal/internal/session"
)

// claveAlfabet no té lletres que es confonen en escriure-les al mòbil
// (0/o, 1/l/i).
const claveAlfabet = "23456789abcdefghjkmnpqrstuvwxyz"

// serverKeyPath és on es desa la clau generada (al costat de les dades).
func serverKeyPath() string {
	return filepath.Join(filepath.Dir(session.DefaultDir()), "server-key")
}

// novaClau fa una clau de 4 grups de 5 caràcters (~99 bits): prou llarga per
// a internet i fàcil de copiar a mà.
func novaClau() (string, error) {
	var b strings.Builder
	n := big.NewInt(int64(len(claveAlfabet)))
	for i := 0; i < 20; i++ {
		if i > 0 && i%5 == 0 {
			b.WriteByte('-')
		}
		x, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", err
		}
		b.WriteByte(claveAlfabet[x.Int64()])
	}
	return b.String(), nil
}

// claveServidor torna la clau desada o en crea una de nova (0600). Així
// `gregal --serve --addr 0.0.0.0` ja funciona sense haver de triar cap token,
// i la clau no canvia entre arrencades.
func claveServidor() (string, error) {
	path := serverKeyPath()
	if raw, err := os.ReadFile(path); err == nil {
		if k := strings.TrimSpace(string(raw)); k != "" {
			return k, nil
		}
	}
	k, err := novaClau()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return k, os.WriteFile(path, []byte(k+"\n"), 0o600)
}

// adrecesXarxa són les IPv4 d'aquesta màquina a la xarxa (sense loopback).
func adrecesXarxa() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || interficieVirtual(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
					out = append(out, ip4.String())
				}
			}
		}
	}
	return out
}

// interficieVirtual descarta les xarxes de màquines virtuals i contenidors
// (WSL, Docker, Hyper-V…): el mòbil no hi arriba i només confonen.
func interficieVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, v := range []string{"vethernet", "wsl", "docker", "veth", "br-", "virbr", "vmnet", "vmware", "virtualbox", "vbox", "hyper-v", "utun", "awdl", "llw"} {
		if strings.Contains(n, v) {
			return true
		}
	}
	return false
}

// infoConnexio és el missatge per a la persona que engega el servidor.
// publicURL ve de GREGAL_PUBLIC_URL (el domini amb HTTPS de l'instal·lador);
// lan, de adrecesXarxa (es passa per poder-ho provar).
func infoConnexio(lang, host, port, key, publicURL string, ambUsuaris bool, lan []string) string {
	ca := lang == "ca"
	t := func(c, e string) string {
		if ca {
			return c
		}
		return e
	}
	var b strings.Builder
	b.WriteString("\n" + t("Gregal està en marxa.", "Gregal is running.") + "\n")
	local := nomesLocal(host)
	if local && publicURL == "" {
		b.WriteString(t(
			"Només aquest ordinador hi pot entrar. Per connectar-hi el mòbil, engega'l amb:\n  gregal --serve --addr 0.0.0.0\n",
			"Only this computer can reach it. To connect the phone, start it with:\n  gregal --serve --addr 0.0.0.0\n"))
		return b.String()
	}
	b.WriteString(t("A l'app del mòbil, «Connecta't a un servidor», escriu:", "In the phone app, \"Connect to a server\", enter:") + "\n")
	if publicURL != "" {
		fmt.Fprintf(&b, "  %-10s %s\n", t("Adreça:", "Address:"), publicURL)
		fmt.Fprintf(&b, "  %-10s %s\n", "Port:", t("(no cal)", "(not needed)"))
	} else {
		adreces := []string{host}
		if host == "" || host == "0.0.0.0" || host == "::" {
			adreces = lan
		}
		if len(adreces) == 0 {
			adreces = []string{t("(la IP d'aquest ordinador)", "(this computer's IP)")}
		}
		fmt.Fprintf(&b, "  %-10s %s\n", t("Adreça:", "Address:"), strings.Join(adreces, t("  o  ", "  or  ")))
		fmt.Fprintf(&b, "  %-10s %s\n", "Port:", port)
	}
	switch {
	case key != "":
		fmt.Fprintf(&b, "  %-10s %s\n", t("Clau:", "Key:"), key)
	case ambUsuaris:
		fmt.Fprintf(&b, "  %s\n", t("i entra amb el teu usuari i contrasenya.", "and sign in with your username and password."))
	}
	if publicURL == "" {
		b.WriteString(t("(Des de fora de casa cal un domini amb HTTPS: vegeu docs/server.ca.md.)\n",
			"(From outside home you need a domain with HTTPS: see docs/server.md.)\n"))
	}
	return b.String()
}
