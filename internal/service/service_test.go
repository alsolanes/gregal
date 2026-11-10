package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gregal/internal/profile"
)

func TestPublishReadAndRemoveNoEsborraUnaInstanciaNova(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	d := Descriptor{Protocol: Protocol, Instance: "inst-vella", Address: "http://127.0.0.1:1234", PID: 42, StartedAt: time.Now()}
	cleanup, err := Publish(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read()
	if err != nil || got.Instance != d.Instance || got.Address != d.Address {
		t.Fatalf("descriptor: %+v %v", got, err)
	}
	newD := d
	newD.Instance = "inst-nova"
	if _, err := Publish(newD); err != nil {
		t.Fatal(err)
	}
	cleanup()
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("el cleanup de la instància vella no pot tocar la nova: %v", err)
	}
	if err := Remove(newD.Instance); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatalf("descriptor encara existeix: %v", err)
	}
}

func TestPublishRebutjaDescriptorIncomplet(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if _, err := Publish(Descriptor{Protocol: Protocol}); err == nil {
		t.Fatal("cal rebutjar un descriptor sense identitat ni adreça")
	}
}

func TestAcquireImpedeixDuesInstancies(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	lease, err := Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if _, err := Acquire(); err == nil {
		t.Fatal("el segon servei hauria de quedar bloquejat")
	}
}

func TestAcquireRecuperaLockOrfe(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(DataDir(), LockName), []byte("99999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire()
	if err != nil {
		t.Fatalf("no ha recuperat el lock orfe: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

// Gregal i Gregal Jocs corren alhora sobre el mateix perfil: cada producte
// té el seu lock i el seu descriptor, i dins d'un mateix producte el segon
// continua bloquejat.
func TestProductesDiferentsConviuen(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	gregal, err := AcquireFor("")
	if err != nil {
		t.Fatal(err)
	}
	defer gregal.Release()
	jocs, err := AcquireFor("jocs")
	if err != nil {
		t.Fatalf("Gregal Jocs ha de poder arrencar amb Gregal en marxa: %v", err)
	}
	defer jocs.Release()
	if _, err := AcquireFor("jocs"); err == nil {
		t.Fatal("un segon Gregal Jocs hauria de quedar bloquejat")
	}
	if _, err := AcquireFor(""); err == nil {
		t.Fatal("un segon Gregal hauria de quedar bloquejat")
	}
	if _, err := os.Stat(filepath.Join(DataDir(), LockName)); err != nil {
		t.Fatalf("Gregal manté service.lock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(DataDir(), "service-jocs.lock")); err != nil {
		t.Fatalf("Gregal Jocs fa servir service-jocs.lock: %v", err)
	}
	if PathFor("") != filepath.Join(DataDir(), DescriptorName) || PathFor("jocs") != filepath.Join(DataDir(), "service-jocs.json") {
		t.Fatalf("descriptors: %s %s", PathFor(""), PathFor("jocs"))
	}

	// Cada procés publica i llegeix el del seu producte.
	defer SetProduct("")
	SetProduct("jocs")
	if _, err := Publish(Descriptor{Protocol: Protocol, Instance: "inst-jocs", Address: "http://127.0.0.1:8197"}); err != nil {
		t.Fatal(err)
	}
	SetProduct("")
	if _, err := Publish(Descriptor{Protocol: Protocol, Instance: "inst-gregal", Address: "http://127.0.0.1:8097"}); err != nil {
		t.Fatal(err)
	}
	if d, err := Read(); err != nil || d.Instance != "inst-gregal" {
		t.Fatalf("Gregal ha de trobar el seu servidor: %+v %v", d, err)
	}
	SetProduct("jocs")
	if d, err := Read(); err != nil || d.Instance != "inst-jocs" {
		t.Fatalf("Gregal Jocs ha de trobar el seu servidor: %+v %v", d, err)
	}
}

// Sense GREGAL_DATA_DIR, el lock i el descriptor de Gregal Jocs van a la
// carpeta de dades del seu perfil (~/.local/share/gregal-jocs), no a la de
// Gregal.
func TestGregalJocsTeLaSevaCarpetaDeDades(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GREGAL_DATA_DIR", "")
	defer profile.SetProduct(profile.Product())
	profile.SetProduct("jocs")
	defer SetProduct("")
	SetProduct("jocs")
	want := filepath.Join(home, ".local", "share", "gregal-jocs")
	if DataDir() != want || Path() != filepath.Join(want, "service-jocs.json") {
		t.Fatalf("Jocs: %s %s", DataDir(), Path())
	}
	profile.SetProduct("")
	if DataDir() != filepath.Join(home, ".local", "share", "gregal") {
		t.Fatalf("Gregal: %s", DataDir())
	}
}
