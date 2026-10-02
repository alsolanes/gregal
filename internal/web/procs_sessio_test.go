package web

import (
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// Els processos de segon pla que engega el MODEL han de quedar etiquetats
// amb la sessió que els ha demanat. Abans duien tots "agent" i, com que el
// Terminal llista per id de sessió, no sortien enlloc — i tancar la
// pestanya tampoc no els matava.
func TestProcessosEtiquetatsAmbLaSessio(t *testing.T) {
	s := &Server{cfg: &config.Config{}, cwd: ".", mode: "code", id: "pestanya-a"}
	args, _ := json.Marshal(map[string]string{"command": "echo hola"})
	c := crida("bash_background", string(args))
	out, _ := s.execTool(c)
	if strings.HasPrefix(out, "ERROR") {
		t.Fatalf("no s'ha engegat: %q", out)
	}
	t.Cleanup(func() { agent.Procs().KillSession("pestanya-a") })

	meus := agent.Procs().List("pestanya-a")
	if len(meus) == 0 {
		t.Fatal("el procés no surt a la seva sessió: és invisible al Terminal de la pestanya")
	}
	// I no ha de sortir a una altra pestanya.
	if altres := agent.Procs().List("pestanya-b"); len(altres) != 0 {
		t.Fatalf("surt en una sessió que no és la seva: %+v", altres)
	}
	// Tancar la sessió se'l pot emportar (abans quedava penjat).
	if n := agent.Procs().KillSession("pestanya-a"); n < 0 {
		t.Fatal("KillSession hauria de poder-hi arribar")
	}
}
