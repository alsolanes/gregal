package agent

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"gregal/internal/gdocs"
	"gregal/internal/tools"
)

func TestEinaDoc(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	out, err := docExec(dir, `{"action":"create","title":"Pla","md":"# Pla\n\nPrimer paràgraf."}`)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\[Pla\]\(/api/docs/([a-z0-9]+)/export\?format=docx\)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("sense enllaç de baixada:\n%s", out)
	}
	id := m[1]
	s, _ := gdocs.Open(dir)
	d, _ := s.Get(id)
	if len(d.Blocks) != 2 {
		t.Fatalf("%+v", d.Blocks)
	}
	call := func(args map[string]any) string {
		t.Helper()
		args["id"] = id
		raw, _ := json.Marshal(args)
		out, err := docExec(dir, string(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		return out
	}
	call(map[string]any{"action": "insert", "after": d.Blocks[1].ID, "md": "Segon."})
	call(map[string]any{"action": "replace", "block": d.Blocks[1].ID, "md": "Primer, reescrit."})
	call(map[string]any{"action": "delete", "block": d.Blocks[0].ID})
	read := call(map[string]any{"action": "read"})
	if strings.Contains(read, "# Pla") || !strings.Contains(read, "Primer, reescrit.") || !strings.Contains(read, "Segon.") ||
		!strings.Contains(read, "["+d.Blocks[1].ID+" v3]") {
		t.Fatalf("read:\n%s", read)
	}
	if strings.Index(read, "Primer, reescrit.") > strings.Index(read, "Segon.") {
		t.Fatalf("ordre:\n%s", read)
	}
	if list, _ := docExec(dir, `{"action":"list"}`); !strings.Contains(list, id) || !strings.Contains(list, "Pla") {
		t.Fatalf("list:\n%s", list)
	}
	if !tools.EsNativa("doc") {
		t.Fatal("doc no és al registre d'eines")
	}
}

func TestEinaDocEnModes(t *testing.T) {
	p := &Policy{}
	args := `{"action":"replace","id":"x","block":"y","md":"z"}`
	if d, why := p.Decide(ModeCode, "doc", args); d != "allow" {
		t.Fatalf("codi: %s %s", d, why)
	}
	for _, m := range []string{ModeChat, ModePlan} {
		if d, _ := p.Decide(m, "doc", args); d != "deny" {
			t.Fatalf("%s: %s", m, d)
		}
	}
}
