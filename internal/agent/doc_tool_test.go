package agent

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"gregal/internal/gdocs"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// tornDocs arrenca un torn de la sessió al projecte dir i torna el que
// rep el model al primer pas i l'historial que es desaria.
func tornDocs(t *testing.T, sessio, dir string) (model string, desat []llm.Message) {
	t.Helper()
	tn := NouTorn(OpcionsTorn{
		Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 5, Sessio: sessio, Workspace: dir,
		Hist:      []llm.Message{{Role: "user", Content: "adapt the rest"}},
		SysPrompt: func() string { return "sys" },
	})
	p := tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("ordre %v", p.Ordre)
	}
	var b strings.Builder
	for _, m := range p.Hist {
		b.WriteString(m.Content + "\n")
	}
	return b.String(), tn.Hist
}

// creaDoc fa un document amb l'eina, com l'agent de la sessió.
func creaDoc(t *testing.T, dir, sessio, md string) gdocs.Doc {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"action": "create", "title": "Plan", "md": md})
	out, err := docExec(dir, sessio, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := gdocs.Open(dir)
	d, err := s.Get(regexp.MustCompile(`/api/docs/([a-z0-9]+)/`).FindStringSubmatch(out)[1])
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTornPortaElsCanvisDeLaPersona(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	d := creaDoc(t, dir, "s1", "# Plan\n\nIntro.\n\nRest.")
	if model, _ := tornDocs(t, "s1", dir); strings.Contains(model, MarcaDocs) {
		t.Fatalf("nota sense canvis de la persona:\n%s", model)
	}
	s, _ := gdocs.Open(dir)
	edit, esborrat := d.Blocks[1].ID, d.Blocks[2].ID
	if _, err := s.Replace(d.ID, edit, "New intro.", gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(d.ID, esborrat, gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert(d.ID, edit, "Added by hand.", gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	model, desat := tornDocs(t, "s1", dir)
	for _, want := range []string{MarcaDocs, "Plan", "edited " + edit + ": New intro.", "deleted " + esborrat, "inserted ", "Added by hand."} {
		if !strings.Contains(model, want) {
			t.Fatalf("falta %q a l'entrada del model:\n%s", want, model)
		}
	}
	for _, m := range desat {
		if strings.Contains(m.Content, MarcaDocs) {
			t.Fatalf("la nota ha anat a l'historial: %+v", desat)
		}
	}
	if model, _ := tornDocs(t, "s1", dir); strings.Contains(model, MarcaDocs) {
		t.Fatalf("els mateixos canvis, dos cops:\n%s", model)
	}
}

func TestTornNoReportaDocumentsQueLaSessioNoHaTocat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	d := creaDoc(t, dir, "s1", "Intro.")
	s, _ := gdocs.Open(dir)
	if _, err := s.Replace(d.ID, d.Blocks[0].ID, "New intro.", gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	if model, _ := tornDocs(t, "s2", dir); strings.Contains(model, MarcaDocs) {
		t.Fatalf("una sessió que no l'ha tocat el veu:\n%s", model)
	}
	if model, _ := tornDocs(t, "s1", dir); !strings.Contains(model, "New intro.") {
		t.Fatalf("la sessió que l'ha escrit no el veu:\n%s", model)
	}
}

func TestEinaDoc(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	out, err := docExec(dir, "s1", `{"action":"create","title":"Pla","md":"# Pla\n\nPrimer paràgraf."}`)
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
		out, err := docExec(dir, "s1", string(raw))
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
	if list, _ := docExec(dir, "s1", `{"action":"list"}`); !strings.Contains(list, id) || !strings.Contains(list, "Pla") {
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
