package goal

import (
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/backlog"
	"gregal/internal/llm"
)

const blocNou = "Ho tinc clar.\n\n```goal\n" +
	"type: bug\n" +
	"title: Export does not crash on an empty month\n" +
	"why: Accounting loses the report.\n" +
	"Steps: open May, click Export.\n" +
	"criteria:\n" +
	"- Exporting an empty month gives a header-only file\n" +
	"- No error dialog\n" +
	"out_of_scope: PDF export\n" +
	"size: s\n" +
	"```\nÉs correcte?"

func TestParseItemFormatNou(t *testing.T) {
	it, ok := ParseItem(blocNou)
	if !ok {
		t.Fatal("no ha trobat el bloc")
	}
	if it.Type != "bug" || it.Title != "Export does not crash on an empty month" || it.Size != "S" || it.OutOfScope != "PDF export" {
		t.Fatalf("%+v", it)
	}
	if it.Why != "Accounting loses the report.\nSteps: open May, click Export." {
		t.Fatalf("el perquè ha de recollir les línies que el segueixen: %q", it.Why)
	}
	if len(it.Criteria) != 2 || it.Criteria[1].Text != "No error dialog" {
		t.Fatalf("%+v", it.Criteria)
	}
}

func TestParseItemFormatVell(t *testing.T) {
	it, ok := ParseItem(replyAmbBloc)
	if !ok || it.Title != "Afegir scroll al TUI" || it.Type != "feature" {
		t.Fatalf("%v %+v", ok, it)
	}
	if len(it.Criteria) != 2 || !strings.Contains(it.Why, "Plan:\n- tocar refresh()") || !strings.Contains(it.Why, "Risks:\n- perdre l'autoscroll") {
		t.Fatalf("els passos i els riscos van al perquè: %q", it.Why)
	}
}

// desaBloc desa el bloc i exigeix que vagi bé.
func desaBloc(t *testing.T, bloc, dir string) (backlog.Item, bool) {
	t.Helper()
	it, created, err := Record(bloc, dir)
	if err != nil {
		t.Fatal(err)
	}
	return it, created
}

func TestRecordCreaIAnotaElNumero(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	it, created := desaBloc(t, blocNou, t.TempDir())
	if !created || it.ID != 1 || it.Status != backlog.StatusProposed {
		t.Fatalf("%v %+v", created, it)
	}
	// La nota diu el número i com revisar-la; queda a la resposta.
	convo := []llm.Message{{Role: "user", Content: "x"}, {Role: "assistant", Content: "bloc"}}
	AnnotateLast(convo, Note(it, created))
	if !strings.Contains(convo[1].Content, "#1 (proposed)") || !strings.Contains(convo[1].Content, "id: 1") {
		t.Fatalf("%q", convo[1].Content)
	}
}

func TestRecordRevisaFinsQueEsComenca(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	desaBloc(t, blocNou, dir)
	// Amb id: es revisa la mateixa, no se'n fa una altra.
	revisat := strings.Replace(blocNou, "```goal\n", "```goal\nid: 1\n", 1)
	revisat = strings.Replace(revisat, "- No error dialog\n", "- No error dialog\n- The file name has the month\n", 1)
	if it, created := desaBloc(t, revisat, dir); created || it.ID != 1 || len(it.Criteria) != 3 {
		t.Fatalf("%v %+v", created, it)
	}
	// Començada, ja no es reescriu: un bloc amb el seu id en fa una de nova.
	if _, task, err := Start(dir, 1, "you"); err != nil || !strings.Contains(task, "backlog item #1") {
		t.Fatalf("%v %q", err, task)
	}
	if it, created := desaBloc(t, revisat, dir); !created || it.ID != 2 {
		t.Fatalf("una tasca començada no es toca: %+v", it)
	}
	if got, _ := Resolve(dir, "#1"); got.Status != backlog.StatusInProgress || got.Workdir == "" {
		t.Fatalf("Start ha de deixar-la en curs i amb el lloc: %+v", got)
	}
}

func TestMigracioDObjectius(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	cfgDir, proj := t.TempDir(), t.TempDir()
	for _, g := range []Goal{
		{ID: "a1", CWD: proj, Title: "Afegir scroll al TUI", Body: replyAmbBloc[strings.Index(replyAmbBloc, "tasca:"):strings.LastIndex(replyAmbBloc, "```")], Status: StatusEsborrany, Created: time.Now()},
		{ID: "a2", CWD: proj, Title: "Ja fet", Body: "task: x", Status: StatusFet},
		{ID: "a3", CWD: proj + "-no-existeix", Title: "Orfe", Body: "task: y"},
	} {
		if err := Save(cfgDir, g); err != nil {
			t.Fatal(err)
		}
	}
	n, err := MigrateToBacklog(cfgDir)
	if err != nil || n != 1 {
		t.Fatalf("%v %d", err, n)
	}
	open, _ := Open(proj)
	if len(open) != 1 || open[0].Title != "Afegir scroll al TUI" || len(open[0].Criteria) != 2 || open[0].Log[0].Who != "migrated from goals" {
		t.Fatalf("%+v", open)
	}
	if _, err := os.Stat(Path(cfgDir)); !os.IsNotExist(err) {
		t.Fatal("goals.json s'ha d'apartar perquè no es torni a migrar")
	}
	if n, err := MigrateToBacklog(cfgDir); n != 0 || err != nil {
		t.Fatalf("la segona vegada no fa res: %d %v", n, err)
	}
}

// El bloc pot arribar abans d'una pregunta: la resposta final ja no el
// porta. Un bloc d'un torn anterior no compta.
func TestTurnReply(t *testing.T) {
	bloc := "Here it is.\n```goal\ntitle: T\n```"
	hist := []llm.Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "```goal\ntitle: Old\n```"},
		{Role: "user", Content: "now this"},
		{Role: "assistant", Content: bloc},
		{Role: "tool", Content: "answer: yes"},
		{Role: "assistant", Content: "Saved."},
	}
	if got := TurnReply(hist, "Saved."); got != bloc {
		t.Fatalf("ha de trobar el bloc del torn: %q", got)
	}
	if got := TurnReply(hist[:3], "No block."); got != "No block." {
		t.Fatalf("un bloc d'un torn anterior no es torna a desar: %q", got)
	}
}

// Si el model ja l'ha creada amb l'eina dins del torn (o repeteix el bloc
// sense id), el bloc la revisa: no hi ha duplicat.
func TestRecordNoDuplicaPelTitol(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	st, err := backlog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(backlog.Item{Type: "bug", Title: "Export does not crash on an  empty month"}, "agent"); err != nil {
		t.Fatal(err)
	}
	it, created, err := Record(blocNou, dir)
	if err != nil || created || it.ID != 1 || len(it.Criteria) != 2 {
		t.Fatalf("%v %v %+v", err, created, it)
	}
	all, _ := st.List()
	if len(all) != 1 {
		t.Fatalf("duplicada: %d", len(all))
	}
}
