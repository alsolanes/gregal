package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseApproved(t *testing.T) {
	v := ParseVerdict("VEREDICTE: APROVAT\nTot bé.")
	if !v.Approved {
		t.Fatal("havia de ser aprovat")
	}
}

func TestParseRevisions(t *testing.T) {
	v := ParseVerdict("VEREDICTE: CAL REVISAR\n- bug a la línia 3")
	if v.Approved {
		t.Fatal("havia de demanar revisió")
	}
}

// El prompt actual demana la línia en anglès; la catalana d'abans continua
// valent (revisors antics, logs reprocessats).
func TestParseVerdictAngles(t *testing.T) {
	if v := ParseVerdict("VERDICT: APPROVED\nLooks good."); !v.Approved {
		t.Fatal("VERDICT: APPROVED ha de ser aprovat")
	}
	if v := ParseVerdict("VERDICT: NEEDS REVISION\n- bug on line 3"); v.Approved {
		t.Fatal("VERDICT: NEEDS REVISION ha de demanar revisió")
	}
}

func TestParseStrictWithoutVerdict(t *testing.T) {
	v := ParseVerdict("Em sembla bé en general.")
	if v.Approved {
		t.Fatal("sense veredicte explícit no s'aprova")
	}
}

func TestBuildPromptHasRubric(t *testing.T) {
	p := BuildPrompt("user: fes X\nassistant: fet", "diff --git ...", "en")
	for _, want := range []string{"VERDICT: APPROVED", "RUBRIC", "EXCHANGE", "DIFF", "LANGUAGE: Always reply to the user in English"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt sense %q", want)
		}
	}
	// Ja no força el català: la llengua dels motius és la de la sessió.
	if strings.Contains(p, "català") {
		t.Fatal("el prompt en anglès no ha de demanar català")
	}
	if ca := BuildPrompt("x", "", "ca"); !strings.Contains(ca, "Respon sempre a l'usuari en català") {
		t.Fatalf("amb lang ca els motius han de ser en català:\n%s", ca)
	}
}

// logVerdict escriu una línia JSON amb model + veredicte (D2).
func TestLogVerdict(t *testing.T) {
	p := filepath.Join(t.TempDir(), "verify-log.jsonl")
	t.Setenv("GREGAL_VERIFY_LOG", p)
	logVerdict("model-prova", Verdict{Approved: true})
	logVerdict("model-prova", Verdict{Approved: false})
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("línies=%d: %q", len(lines), raw)
	}
	var r map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &r); err != nil {
		t.Fatal(err)
	}
	if r["model"] != "model-prova" || r["verdict"] != "APROVAT" || r["ts"] == "" {
		t.Fatalf("registre=%v", r)
	}
}

func TestDidRealWork(t *testing.T) {
	if DidRealWork(3, 3, "") {
		t.Fatal("sense escriptures ni diff = sense feina real")
	}
	if DidRealWork(3, 3, "  \n ") {
		t.Fatal("diff en blanc = sense feina real")
	}
	if !DidRealWork(3, 4, "") {
		t.Fatal("journal crescut = feina real")
	}
	if !DidRealWork(3, 3, "M fitxer.go") {
		t.Fatal("diff no buit = feina real")
	}
}

func TestBuildPromptAntiSoroll(t *testing.T) {
	p := BuildPrompt("usuari: hola", "", "en")
	if !strings.Contains(p, "ANTI-NOISE") {
		t.Fatal("el prompt ha de prohibir CAL REVISAR sense feina")
	}
}
