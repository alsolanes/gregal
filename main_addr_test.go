package main

import "testing"

func TestAdrecaEscolta(t *testing.T) {
	casos := []struct{ addr, port, vol string }{
		{"", "8097", "127.0.0.1:8097"},
		{"0.0.0.0", "9000", "0.0.0.0:9000"},
		{" 127.0.0.1 ", "8097", "127.0.0.1:8097"},
		// El cas que petava: l'adreça ja porta port.
		{"0.0.0.0:9000", "8097", "0.0.0.0:9000"},
		{":9000", "8097", "127.0.0.1:9000"},
		// IPv6, que també porta dos punts però no són un port.
		{"::1", "8097", "[::1]:8097"},
		{"[::1]:9000", "8097", "[::1]:9000"},
	}
	for _, c := range casos {
		if got := adrecaEscolta(c.addr, c.port); got != c.vol {
			t.Errorf("adrecaEscolta(%q, %q) = %q, volem %q", c.addr, c.port, got, c.vol)
		}
	}
}

func TestNomesLocal(t *testing.T) {
	per := map[string]bool{
		"127.0.0.1": true, "127.0.0.2": true, "localhost": true, "::1": true,
		"0.0.0.0": false, "192.168.1.10": false, "": false,
	}
	for host, vol := range per {
		if got := nomesLocal(host); got != vol {
			t.Errorf("nomesLocal(%q) = %v, volem %v", host, got, vol)
		}
	}
}
