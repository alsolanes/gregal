package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
