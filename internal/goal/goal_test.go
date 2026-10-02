package goal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const replyAmbBloc = "Abans de res, dues coses que em calen.\n\n" +
	"```goal\n" +
	"tasca: Afegir scroll al TUI\n" +
	"context: l'historial no es pot moure amb la roda\n" +
	"criteris:\n" +
	"- la roda mou el viewport\n" +
	"- PageUp/PageDown funcionen\n" +
	"passos:\n" +
	"- tocar refresh()\n" +
	"- afegir bindings\n" +
	"riscos:\n" +
	"- perdre l'autoscroll\n" +
	"```\n"

func TestParseTrobaElBloc(t *testing.T) {
	g, ok := Parse(replyAmbBloc, "/home/usera/tui-agent")
	if !ok {
		t.Fatal("esperava trobar un bloc goal")
	}
	if g.Title != "Afegir scroll al TUI" {
		t.Fatalf("title=%q", g.Title)
	}
	for _, want := range []string{"la roda mou el viewport", "PageUp/PageDown funcionen", "tocar refresh()"} {
		if !strings.Contains(g.Body, want) {
			t.Fatalf("el cos no conté %q:\n%s", want, g.Body)
		}
	}
	if strings.Contains(g.Body, "```") {
		t.Fatalf("el cos no ha de conservar les tanques del bloc:\n%s", g.Body)
	}
	if g.Project == "" {
		t.Fatal("project buit")
	}
	if g.ID == "" || g.Created.IsZero() {
		t.Fatalf("id/created buits: %+v", g)
	}
	if g.Status != StatusEsborrany {
		t.Fatalf("status=%q, volia %q", g.Status, StatusEsborrany)
	}
}

func TestParseSenseBloc(t *testing.T) {
	if _, ok := Parse("Segur que vols això? Quantes carpetes vols cobrir?", "/tmp/p"); ok {
		t.Fatal("no esperava cap objectiu")
	}
}

func TestParseBlocBuit(t *testing.T) {
	if _, ok := Parse("```goal\n```\n", "/tmp/p"); ok {
		t.Fatal("un bloc buit no és un objectiu")
	}
}

func TestParseIDEstable(t *testing.T) {
	a, _ := Parse(replyAmbBloc, "/tmp/p")
	b, _ := Parse(replyAmbBloc, "/tmp/p")
	if a.ID == b.ID {
		t.Fatal("dos objectius del mateix text han de tenir ids diferents")
	}
}

func TestSaveListDelete(t *testing.T) {
	dir := t.TempDir()
	primer := Goal{ID: "aaa111", Created: time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC),
		Project: "tui-agent", CWD: "/home/usera/tui-agent", Title: "primer", Body: "tasca: primer"}
	segon := Goal{ID: "bbb222", Created: time.Date(2026, 9, 12, 11, 0, 0, 0, time.UTC),
		Project: "tui-agent", CWD: "/home/usera/tui-agent", Title: "segon", Body: "tasca: segon"}
	if err := Save(dir, primer); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, segon); err != nil {
		t.Fatal(err)
	}
	llista, err := List(dir, "tui-agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(llista) != 2 {
		t.Fatalf("esperava 2 objectius, tinc %d", len(llista))
	}
	if llista[0].ID != "bbb222" {
		t.Fatalf("el més nou ha d'anar primer, tinc %s", llista[0].ID)
	}
	if altres, _ := List(dir, "altre-projecte"); len(altres) != 0 {
		t.Fatalf("no hauria de tornar objectius d'un altre projecte: %d", len(altres))
	}
	if tots, _ := List(dir, ""); len(tots) != 2 {
		t.Fatalf("sense filtre hauria de tornar-ho tot: %d", len(tots))
	}
	if err := Delete(dir, "aaa111"); err != nil {
		t.Fatal(err)
	}
	resta, _ := List(dir, "")
	if len(resta) != 1 || resta[0].ID != "bbb222" {
		t.Fatalf("després d'esborrar: %+v", resta)
	}
	if err := Delete(dir, "noexisteix"); err == nil {
		t.Fatal("esperava error en esborrar un id inexistent")
	}
}

func TestSaveActualitzaMateixID(t *testing.T) {
	dir := t.TempDir()
	g := Goal{ID: "ccc333", Project: "p", Title: "esborrany", Status: StatusEsborrany}
	if err := Save(dir, g); err != nil {
		t.Fatal(err)
	}
	g.Status = StatusFet
	g.Title = "fet"
	if err := Save(dir, g); err != nil {
		t.Fatal(err)
	}
	llista, _ := List(dir, "")
	if len(llista) != 1 || llista[0].Status != StatusFet || llista[0].Title != "fet" {
		t.Fatalf("esperava un sol objectiu actualitzat: %+v", llista)
	}
}

func TestFitxerAmbPermisos0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows no té permisos POSIX: el mode sempre surt 666")
	}
	dir := t.TempDir()
	if err := Save(dir, Goal{ID: "ddd444", Project: "p", Title: "x"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "goals.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permisos=%o, volia 600", perm)
	}
}

func TestTaskInclouCriteris(t *testing.T) {
	g, _ := Parse(replyAmbBloc, "/home/usera/tui-agent")
	tasca := Task(g)
	for _, want := range []string{"Afegir scroll al TUI", "PageUp/PageDown funcionen", "tui-agent"} {
		if !strings.Contains(tasca, want) {
			t.Fatalf("la tasca no conté %q:\n%s", want, tasca)
		}
	}
}

func TestPromptDemanaPreguntes(t *testing.T) {
	p := Prompt()
	if !strings.Contains(p, "```goal") {
		t.Fatalf("el prompt ha d'explicar el format del bloc:\n%s", p)
	}
	if !strings.Contains(strings.ToLower(p), "pregunt") {
		t.Fatalf("el prompt ha de dir que faci preguntes:\n%s", p)
	}
	// Les preguntes han de sortir seleccionables (eina question), no en
	// text pla: si no, el model interroga amb llistes i el circuit de
	// pregunta/resposta de la UI no s'usa mai.
	if !strings.Contains(p, "question") {
		t.Fatalf("el prompt ha de demanar l'eina question:\n%s", p)
	}
	// I no pot tancar el torn amb text que no porta enlloc: o pregunta
	// (question) o tanca amb el bloc.
	if !strings.Contains(p, "mai amb text pla") {
		t.Fatalf("el prompt ha de prohibir el final en text pla:\n%s", p)
	}
}
