package companion

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)


// TestWindStoreFlockDosProcessos: dues instàncies del store (com el servidor
// i l'escriptori) que escriuen alhora no es perden els canvis: el flock
// serialitza el cicle llegir-modificar-escriure entre processos.
func TestWindStoreFlockDosProcessos(t *testing.T) {
	dir := t.TempDir()
	a, b := NewWindStore(dir), NewWindStore(dir)
	if _, err := a.Save(Wind{Name: "Tramuntana"}); err != nil {
		t.Fatal(err)
	}
	const N = 12
	var wg sync.WaitGroup
	errs := make(chan error, 2*N)
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, err := a.Save(Wind{Name: "Xaloc"}); errs <- err }()
		go func() { defer wg.Done(); _, err := b.Save(Wind{Name: "Llevant"}); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	xaloc, llevant := 0, 0
	for _, w := range list {
		switch w.Name {
		case "Xaloc":
			xaloc++
		case "Llevant":
			llevant++
		}
	}
	if xaloc != 1 || llevant != 1 {
		t.Fatalf("lost update: xaloc=%d llevant=%d (volem 1 i 1)", xaloc, llevant)
	}
	// El fitxer de lock no forma part del catàleg.
	if _, err := os.Stat(filepath.Join(dir, ".winds.lock")); err != nil {
		t.Fatalf("lock file: %v", err)
	}
}
