package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestInfoConnexioDiuQueEscriureAlMobil(t *testing.T) {
	lan := infoConnexio("ca", "0.0.0.0", "8097", "abcde-fghjk-mnpqr-stuvw", "", false, []string{"192.168.1.84"})
	for _, want := range []string{"Adreça:", "192.168.1.84", "Port:", "8097", "Clau:", "abcde-fghjk-mnpqr-stuvw"} {
		if !strings.Contains(lan, want) {
			t.Fatalf("falta %q:\n%s", want, lan)
		}
	}
	pub := infoConnexio("en", "127.0.0.1", "8097", "k", "https://winds.example.com", false, nil)
	if !strings.Contains(pub, "https://winds.example.com") || !strings.Contains(pub, "(not needed)") {
		t.Fatalf("amb domini, l'adreça és el domini i el port no cal:\n%s", pub)
	}
	nomes := infoConnexio("ca", "127.0.0.1", "8097", "", "", false, nil)
	if !strings.Contains(nomes, "--addr 0.0.0.0") || strings.Contains(nomes, "Clau:") {
		t.Fatalf("només local, ha de dir com obrir-lo:\n%s", nomes)
	}
	usuaris := infoConnexio("ca", "0.0.0.0", "8097", "", "", true, []string{"10.0.0.2"})
	if !strings.Contains(usuaris, "usuari i contrasenya") {
		t.Fatalf("amb usuaris:\n%s", usuaris)
	}
}

func TestClauDelServidorEsDesaIEsReaprofita(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	k1, err := claveServidor()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[2-9a-hjkmnp-z]{5}(-[2-9a-hjkmnp-z]{5}){3}$`).MatchString(k1) {
		t.Fatalf("format de la clau: %q", k1)
	}
	k2, _ := claveServidor()
	if k1 != k2 {
		t.Fatal("la clau ha de ser la mateixa entre arrencades")
	}
}
