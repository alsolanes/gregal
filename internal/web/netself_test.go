package web

import "testing"

// Les IP de Docker i els túnels no són la LAN de casa: si l'app les rep, tria
// una adreça on no respon ningú. Aquest servidor en té setze de ponts de Docker.
func TestLanIPsSkipsVirtualInterfaces(t *testing.T) {
	for _, name := range []string{"docker0", "br-3f2a1c", "veth9b2", "virbr0", "tailscale0", "tun0", "zt123"} {
		if !isVirtualIface(name) {
			t.Fatalf("%q és virtual i s'ha de saltar", name)
		}
	}
	for _, name := range []string{"enp5s0", "wlp3s0", "eno1", "eth0", "wlan0"} {
		if isVirtualIface(name) {
			t.Fatalf("%q és una interfície de debò i no s'ha de saltar", name)
		}
	}
}

// El que es publica ha de ser una llista d'IP i, si està configurat, el domini.
func TestSelfNetCarriesPublicHost(t *testing.T) {
	t.Setenv("GREGAL_PUBLIC_HOST", "gregal.example.cat")
	got := selfNet()
	if got["public_host"] != "gregal.example.cat" {
		t.Fatalf("public_host = %v", got["public_host"])
	}
	ips, ok := got["lan_ips"].([]string)
	if !ok {
		t.Fatalf("lan_ips no és una llista: %T", got["lan_ips"])
	}
	for _, ip := range ips {
		if ip == "127.0.0.1" || ip == "::1" {
			t.Fatalf("s'ha publicat el loopback: %v", ips)
		}
	}
}
