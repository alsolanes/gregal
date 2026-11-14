package web

import (
	"strings"
	"testing"
)

// El torn de Gregal ha d'avisar igual que un vent: si l'app és fora, el mòbil
// ho ha de saber. La novetat és el camí que ja porta push, sondeig i dedupe.
func TestRecordTornAcabat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	s.recordTornAcabat("  He mirat els bitllets: el més barat surt a les 7:15.  ")

	got := getNovetats(t, s, 0)
	if len(got) != 1 {
		t.Fatalf("novetats = %d, calia 1", len(got))
	}
	n := got[0]
	if n.Source != "torn" || n.Wind != gregalWindID || n.Name != "Gregal" {
		t.Fatalf("novetat del torn = %+v", n)
	}
	if n.Text != "He mirat els bitllets: el més barat surt a les 7:15." {
		t.Fatalf("text = %q (calia sense espais als caps)", n.Text)
	}
	if n.Quiet {
		t.Fatal("quiet: el torn que has demanat ha de sonar")
	}
	if n.ID == "" || n.Ts <= 0 {
		t.Fatalf("id/ts buits: %+v", n)
	}
	// Un torn no és una targeta per decidir: no ha d'omplir la safata «Avui».
	if n.Card != "" {
		t.Fatalf("card = %q, el torn no n'ha de portar", n.Card)
	}
}

func TestRecordTornAcabatBuit(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	s.recordTornAcabat("   ")
	if got := getNovetats(t, s, 0); len(got) != 0 {
		t.Fatalf("un torn sense resposta no ha de deixar novetat: %+v", got)
	}
}

// Una resposta llarga no ha d'omplir l'avís: el fil sencer ja és a l'app.
func TestRecordTornAcabatEscurça(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	s.recordTornAcabat(strings.Repeat("a", tornAvísMax+50))
	got := getNovetats(t, s, 0)
	if len(got) != 1 {
		t.Fatalf("novetats = %d", len(got))
	}
	if r := []rune(got[0].Text); len(r) != tornAvísMax+1 || !strings.HasSuffix(got[0].Text, "…") {
		t.Fatalf("llargada %d (calia %d amb «…»)", len(r), tornAvísMax+1)
	}
}
