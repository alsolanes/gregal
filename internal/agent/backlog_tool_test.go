package agent

import (
	"strings"
	"testing"
)

func TestEinaBacklog(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	out, err := backlogExec(dir, "s-1", `{"action":"create","type":"bug","title":"Crash on save","why":"Data loss","criteria":["Saving an empty file works"],"size":"S"}`)
	if err != nil || !strings.Contains(out, "#1 [bug] Crash on save · proposed") {
		t.Fatalf("%v %q", err, out)
	}
	if _, err := backlogExec(dir, "s-1", `{"action":"move","id":1,"status":"ready"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := backlogExec(dir, "s-1", `{"action":"move","id":1,"status":"in progress"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := backlogExec(dir, "s-1", `{"action":"check","id":1,"n":1,"evidence":"TestSaveEmpty"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := backlogExec(dir, "s-1", `{"action":"move","id":1,"status":"done"}`); err == nil || !strings.Contains(err.Error(), "in_review") {
		t.Fatalf("l'agent no tanca tasques: %v", err)
	}
	// Editar els criteris sense canviar-ne el text conserva la prova.
	if _, err := backlogExec(dir, "s-1", `{"action":"edit","id":1,"criteria":["Saving an empty file works","No error dialog"]}`); err != nil {
		t.Fatal(err)
	}
	md, _ := backlogExec(dir, "", `{"action":"get","id":1}`)
	if !strings.Contains(md, "- [x] Saving an empty file works") || !strings.Contains(md, "evidence: TestSaveEmpty") || !strings.Contains(md, "agent · s-1") {
		t.Fatalf("get:\n%s", md)
	}
	list, _ := backlogExec(dir, "", `{"action":"list"}`)
	if !strings.Contains(list, "1/2 criteria") {
		t.Fatalf("list:\n%s", list)
	}
}

func TestEinaBacklogEnModesDeLectura(t *testing.T) {
	p := &Policy{}
	if d, _ := p.Decide(ModeChat, "backlog", `{"action":"create","type":"task","title":"x"}`); d != "allow" {
		t.Fatalf("crear en xat: %s", d)
	}
	if d, _ := p.Decide(ModeChat, "backlog", `{"action":"move","id":1,"status":"ready"}`); d != "deny" {
		t.Fatalf("moure en xat: %s", d)
	}
	if d, _ := p.Decide(ModeCode, "backlog", `{"action":"move","id":1,"status":"ready"}`); d != "allow" {
		t.Fatalf("moure en codi: %s", d)
	}
}
