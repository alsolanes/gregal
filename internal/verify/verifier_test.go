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

func TestParseStrictWithoutVerdict(t *testing.T) {
	v := ParseVerdict("Em sembla bé en general.")
	if v.Approved {
		t.Fatal("sense veredicte explícit no s'aprova")
	}
}

func TestBuildPromptHasRubric(t *testing.T) {
	p := BuildPrompt("user: fes X\nassistant: fet", "diff --git ...")
	for _, want := range []string{"VEREDICTE", "RÚBRICA", "INTERCANVI", "DIFF"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt sense %q", want)
		}
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
	p := BuildPrompt("usuari: hola", "")
	if !strings.Contains(p, "ANTI-SOROLL") {
		t.Fatal("el prompt ha de prohibir CAL REVISAR sense feina")
	}
}
