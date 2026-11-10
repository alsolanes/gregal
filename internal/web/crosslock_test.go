package web

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/companion"
)

// crossLockWrites apunta n veus proactives (i les seves targetes d'«Avui»)
// amb un prefix: és el que fan Gregal i la lliga de Gregal Jocs alhora.
func crossLockWrites(t *testing.T, s *Server, prefix string, n int) {
	for i := 0; i < n; i++ {
		recordNovetat(s, companion.Wind{ID: "xaloc", Name: "Xaloc"}, "football", fmt.Sprintf("%s %d. Final del partit.", prefix, i))
	}
}

// El procés fill del test de sota: escriu al mateix perfil que el pare.
func TestCrossLockHelperProcess(t *testing.T) {
	if os.Getenv("GREGAL_CROSSLOCK_CHILD") != "1" {
		t.Skip("només com a procés fill")
	}
	t.Setenv("GREGAL_DATA_DIR", os.Getenv("GREGAL_CROSSLOCK_DIR"))
	crossLockWrites(t, goalTestServer(t), "fill", 45)
}

// Dos processos (com Gregal i Gregal Jocs sobre el mateix perfil) afegeixen
// veus i targetes al mateix fitxer alhora: no se'n perd cap.
func TestCrossLockDosProcessosNoPerdenEscriptures(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", dir)
	s := goalTestServer(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrossLockHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), "GREGAL_CROSSLOCK_CHILD=1", "GREGAL_CROSSLOCK_DIR="+dir)
	var out []byte
	var cerr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); out, cerr = cmd.CombinedOutput() }()
	// Comença quan el fill ja escriu, perquè les escriptures es creuin.
	for deadline := time.Now().Add(30 * time.Second); len(loadNovetats(s)) == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	crossLockWrites(t, s, "pare", 45)
	wg.Wait()
	if cerr != nil {
		t.Fatalf("procés fill: %v\n%s", cerr, out)
	}
	// Només es compten les d'aquest test: una feina de fons d'un altre test
	// del paquet hi pot deixar alguna veu de més.
	mine := func(text string) bool { return strings.HasPrefix(text, "pare ") || strings.HasPrefix(text, "fill ") }
	got := 0
	for _, n := range loadNovetats(s) {
		if mine(n.Text) {
			got++
		}
	}
	if got != 90 {
		t.Fatalf("veus: %d, volia 90 (s'han perdut escriptures entre processos)", got)
	}
	got = 0
	for _, c := range loadAvui(s) {
		if mine(c.Body) || mine(c.Title) {
			got++
		}
	}
	if got != 90 {
		t.Fatalf("targetes d'Avui: %d, volia 90", got)
	}
}
