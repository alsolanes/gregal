package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// Cada clau que el motor emet té text per defecte: un client sense
// diccionari (Telegram) no pot ensenyar «app.tornBuit» en cru.
func TestTotsElsEventsTenenText(t *testing.T) {
	_, this, _, _ := runtime.Caller(0)
	dir := filepath.Dir(this)
	re := regexp.MustCompile(`Clau\("([a-zA-Z.]+)"`)
	for _, f := range []string{"motor.go", "motor_aux.go"} {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			if _, ok := textosMotor[m[1]]; !ok {
				t.Errorf("la clau %q de %s no té text a textosMotor", m[1], f)
			}
		}
	}
	e := Event{Tipus: EvNota, Clau: "app.tornBuit", Args: []any{3, 40}}
	if got := e.Missatge(); got != "el model ha acabat el torn sense resposta (pas 3/40): torna-ho a demanar o puja max_tokens" {
		t.Fatalf("Missatge: %q", got)
	}
}
