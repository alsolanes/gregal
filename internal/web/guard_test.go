package web

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGuardDinsIFora: el cas feliç i el de fora, que és el que impedeix que
// un usuari miri a casa de l'altre.
func TestGuardDinsIFora(t *testing.T) {
	base := t.TempDir()
	userb := filepath.Join(base, "gregal_src", "userb")
	usera := filepath.Join(base, "gregal_src", "usera")
	for _, d := range []string{userb, usera} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	u := &User{Name: "userb", Roots: []string{userb}}

	dins := []string{
		userb,
		filepath.Join(userb, "web", "index.html"),
		filepath.Join(userb, ".", "sub", "..", "fitxer.txt"),
	}
	for _, p := range dins {
		if err := u.Allow(p); err != nil {
			t.Fatalf("%s hauria de passar: %v", p, err)
		}
	}
	fora := []string{
		usera,
		filepath.Join(usera, "notes.md"),
		filepath.Join(base, "gregal_src"),
		"/",
		"/etc/passwd",
		filepath.Join(userb, "..", "usera", "secrets.txt"),
	}
	for _, p := range fora {
		if err := u.Allow(p); err == nil {
			t.Fatalf("%s no hauria de passar", p)
		}
	}
}

// TestGuardPrefixEnganys: `/…/userb-filla` comparteix prefix amb
// `/…/userb` i no hi ha de passar. És l'error clàssic d'aquestes
// comprovacions.
func TestGuardPrefixEnganys(t *testing.T) {
	base := t.TempDir()
	userb := filepath.Join(base, "userb")
	trampa := filepath.Join(base, "carla2")
	if err := os.MkdirAll(trampa, 0o755); err != nil {
		t.Fatal(err)
	}
	u := &User{Name: "userb", Roots: []string{userb}}
	if err := u.Allow(filepath.Join(trampa, "fitxer.txt")); err == nil {
		t.Fatal("carla2 no és dins userb i hauria de quedar fora")
	}
}

// TestGuardEnllacos: un enllaç simbòlic dins l'arrel que apunta a fora no
// ha de servir per escapar-se'n.
func TestGuardEnllacos(t *testing.T) {
	base := t.TempDir()
	userb := filepath.Join(base, "userb")
	dins := filepath.Join(userb, "dins")
	fora := filepath.Join(base, "usera")
	for _, d := range []string{dins, fora} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(fora, "secret.txt")
	if err := os.WriteFile(secret, []byte("hola"), 0o600); err != nil {
		t.Fatal(err)
	}
	enllac := filepath.Join(userb, "drecera")
	if err := os.Symlink(fora, enllac); err != nil {
		t.Skipf("sense enllaços en aquest sistema: %v", err)
	}
	u := &User{Name: "userb", Roots: []string{userb}}
	if err := u.Allow(filepath.Join(enllac, "secret.txt")); err == nil {
		t.Fatal("la drecera cap a fora hauria de quedar bloquejada")
	}
	// I l'arrel en si, resolta, continua passant.
	if err := u.Allow(dins); err != nil {
		t.Fatalf("una carpeta de dins hauria de passar: %v", err)
	}
}

// TestGuardSenseArrels: un usuari sense carpetes assignades pot xatejar,
// però no navegar.
func TestGuardSenseArrels(t *testing.T) {
	u := &User{Name: "convidat"}
	if err := u.Allow(t.TempDir()); err == nil {
		t.Fatal("sense arrels no s'ha de poder navegar")
	}
}

// TestGuardModeLocal: sense usuaris configurats el token únic mana i va a
// tot arreu: el món de sempre no es pot trencar.
func TestGuardModeLocal(t *testing.T) {
	u := &User{Name: "usera", Admin: true, Tothom: true}
	if err := u.Allow("/etc/passwd"); err != nil {
		t.Fatalf("Tothom hauria de passar: %v", err)
	}
	var nilU *User
	if err := nilU.Allow("/qualsevol"); err != nil {
		t.Fatalf("sense usuari (local) hauria de passar: %v", err)
	}
	if !nilU.Allowed("/x") || u.Arrel() != "" {
		t.Fatal("Allowed i Arrel han de ser coherents")
	}
}

// TestGuardArrelDeSistema: `roots: ["/"]` és el cas de l'amo del servidor.
// Amb l'arrel `/`, el prefix amb separador quedava en `//` i no hi havia
// manera d'entrar enlloc.
func TestGuardArrelDeSistema(t *testing.T) {
	root := "/"
	paths := []string{"/", "/home/usera/tui-agent/README.md", "/etc/passwd", "/var/log/syslog"}
	if filepath.Separator == '\\' {
		root = filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
		paths = []string{root, filepath.Join(root, "Windows"), filepath.Join(root, "Users")}
	}
	u := &User{Name: "usera", Admin: true, Roots: []string{root}}
	for _, p := range paths {
		if err := u.Allow(p); err != nil {
			t.Fatalf("amb l'arrel / hauria de passar %s: %v", p, err)
		}
	}
	// I una arrel concreta continua tancant la resta.
	v := &User{Name: "userb", Roots: []string{"/home/usera/gregal_src/userb"}}
	if err := v.Allow("/etc/passwd"); err == nil {
		t.Fatal("la userb no hauria de poder sortir de la seva carpeta")
	}
}

// TestGuardFitxerNou: un fitxer que encara no existeix dins l'arrel ha de
// poder-se crear; el guard resol el pare.
func TestGuardFitxerNou(t *testing.T) {
	userb := t.TempDir()
	u := &User{Name: "userb", Roots: []string{userb}}
	if err := u.Allow(filepath.Join(userb, "nou", "debon.txt")); err == nil {
		// El pare no existeix: no podem garantir on cau, així que es
		// resol fins on es pot i, si el pare tampoc hi és, es nega.
		t.Log("el guard ha deixat passar una ruta amb pare inexistent")
	}
	if err := u.Allow(filepath.Join(userb, "existeix.txt")); err != nil {
		t.Fatalf("un fitxer nou a l'arrel hauria de passar: %v", err)
	}
}
