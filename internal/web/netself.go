package web

import (
	"net"
	"os"
	"sort"
	"strings"
)

// selfNet diu com es pot arribar a aquest servidor: el domini del túnel
// (GREGAL_PUBLIC_HOST) i les IP de la LAN on també escolta. L'app ho desa en
// connectar-se i després tria sola —a casa la LAN, fora el domini— perquè si
// només coneix l'adreça de casa, fora de casa no respon ningú.
//
// El port no s'hi posa: l'app ja sap amb quin port parla i amb quin va a la LAN.
func selfNet() map[string]any {
	out := map[string]any{
		"public_host": strings.TrimSpace(os.Getenv("GREGAL_PUBLIC_HOST")),
		"lan_ips":     lanIPs(),
	}
	return out
}

// lanIPs són les IPv4 privades d'aquest ordinador, sense loopback ni enllaços
// locals. Buit vol dir que no n'hi ha cap (o que no es poden llegir): l'app
// llavors es queda amb l'adreça que li han donat.
func lanIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}
	ips := []string{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Les xarxes de Docker i els túnels no són la LAN de casa: donar-les a
		// l'app seria donar-li una adreça on no respon ningú.
		if isVirtualIface(ifc.Name) {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || !ip4.IsPrivate() {
				continue
			}
			ips = append(ips, ip4.String())
		}
	}
	sort.Strings(ips)
	return ips
}

// isVirtualIface diu si la interfície és d'un contenidor, un pont o un túnel.
func isVirtualIface(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "vmnet", "tun", "tap", "wg", "tailscale", "zt", "cni", "flannel", "lxc"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
