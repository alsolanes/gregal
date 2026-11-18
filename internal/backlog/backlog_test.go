package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nouProjecte(t *testing.T) *Store {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.Storage != StorageApp {
		t.Fatalf("per defecte ha de viure a l'app, no a %s", s.Storage)
	}
	return s
}

// deu torna una funció que exigeix que l'acció vagi bé: deu(t)(s.Move(...)).
func deu(t *testing.T) func(Item, error) Item {
	return func(it Item, err error) Item {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
}

// refusa exigeix que l'acció es rebutgi amb un error que digui want.
func refusa(t *testing.T, want string) func(Item, error) {
	return func(_ Item, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("esperava un error amb %q, tinc %v", want, err)
		}
	}
}

func TestCicleDUnaTasca(t *testing.T) {
	s := nouProjecte(t)
	ok := deu(t)
	it := ok(s.Create(Item{Type: "Feature", Title: "Export CSV", Status: "done"}, "you"))
	if it.ID != 1 || it.Status != StatusProposed || it.Priority != "next" {
		t.Fatalf("una tasca nova és la 1, proposada i next: %+v", it)
	}
	refusa(t, "why")(s.Move(1, "ready", "", "you"))
	ok(s.Edit(1, "you", func(i *Item) {
		i.Why = "Accounting rebuilds it by hand."
		i.Criteria = []Criterion{{Text: "Button exists"}, {Text: "  "}, {Text: "Empty month gives header only"}}
		i.Size = "l"
	}))
	refusa(t, "size L")(s.Move(1, "ready", "", "you"))
	ok(s.Edit(1, "you", func(i *Item) { i.Size = "S" }))
	refusa(t, "only a ready item")(s.Move(1, "in progress", "", "agent"))
	ok(s.Move(1, "ready", "", "you"))
	ok(s.Move(1, "in-progress", "", "agent"))
	refusa(t, "evidence")(s.Check(1, 1, true, "", "agent"))
	ok(s.Check(1, 1, true, "TestExportButton passes", "agent"))
	refusa(t, "only an item in review")(s.Move(1, "done", "", "you"))
	ok(s.Move(1, "in_review", "", "agent"))
	refusa(t, "criterion 2")(s.Move(1, "done", "", "you"))
	ok(s.Check(1, 2, true, "go test ./report -run Empty", "agent"))
	it = ok(s.Move(1, "done", "", "you"))
	if len(it.Criteria) != 2 {
		t.Fatalf("els criteris buits no es desen: %+v", it.Criteria)
	}
	if n := len(it.Log); n < 7 || it.Log[0].Text != "created" || !strings.Contains(it.Log[n-1].Text, "in_review → done") {
		t.Fatalf("el log no recull el cicle: %+v", it.Log)
	}
}

func TestBlocatIDependencies(t *testing.T) {
	s := nouProjecte(t)
	ok := deu(t)
	a := ok(s.Create(Item{Type: "task", Title: "A", Why: "w", Criteria: []Criterion{{Text: "c"}}}, ""))
	b := ok(s.Create(Item{Type: "task", Title: "B", Why: "w", Criteria: []Criterion{{Text: "c"}}, DependsOn: []int{a.ID}}, ""))
	refusa(t, "#99 does not exist")(s.Create(Item{Type: "task", Title: "C", DependsOn: []int{99}}, ""))
	ok(s.Move(b.ID, "ready", "", ""))
	refusa(t, "#1")(s.Move(b.ID, "in_progress", "", ""))
	refusa(t, "needs a reason")(s.Move(b.ID, "blocked", " ", ""))
	if it := ok(s.Move(b.ID, "blocked", "waiting for #1", "")); it.Reason != "waiting for #1" {
		t.Fatalf("%+v", it)
	}
	refusa(t, "not an epic")(s.Create(Item{Type: "task", Title: "D", Parent: a.ID}, ""))
}

func TestMarkdownIdaITornada(t *testing.T) {
	s := nouProjecte(t)
	ok := deu(t)
	ok(s.Create(Item{Type: "bug", Title: "Crash on empty file", Why: "Users lose work.\n\nTwice this week.",
		Criteria: []Criterion{{Text: "Opening an empty file shows an empty editor"}}, OutOfScope: "Binary files"}, "you"))
	ok(s.Check(1, 1, true, "TestEmptyFile", "agent"))
	it := ok(s.AddLog(1, "found the cause:\nnil slice", "agent · s-12"))
	md := Markdown(it)
	back, err := ParseMarkdown(md)
	if err != nil {
		t.Fatalf("%v\n%s", err, md)
	}
	if back.Title != it.Title || back.Why != it.Why || back.OutOfScope != it.OutOfScope ||
		len(back.Criteria) != 1 || !back.Criteria[0].Done || back.Criteria[0].Evidence != "TestEmptyFile" {
		t.Fatalf("no torna igual:\n%s\n%+v", md, back)
	}
	last := back.Log[len(back.Log)-1]
	if last.Who != "agent · s-12" || last.Text != "found the cause: nil slice" {
		t.Fatalf("log: %+v", last)
	}
}

func TestPassarAlRepo(t *testing.T) {
	s := nouProjecte(t)
	ok := deu(t)
	ok(s.Create(Item{Type: "feature", Title: "One"}, ""))
	ok(s.Create(Item{Type: "spike", Title: "Two"}, ""))
	app := s.Path
	if err := s.MoveToRepo(); err != nil {
		t.Fatal(err)
	}
	if s.Storage != StorageRepo {
		t.Fatal("no ha canviat de lloc")
	}
	if _, err := os.Stat(app + ".moved"); err != nil {
		t.Fatal("el JSON antic s'ha de guardar apartat")
	}
	again, err := Open(s.Root)
	if err != nil || again.Storage != StorageRepo {
		t.Fatalf("en tornar a obrir, la carpeta mana: %v", err)
	}
	all, err := again.List()
	if err != nil || len(all) != 2 || all[1].Title != "Two" {
		t.Fatalf("%v %+v", err, all)
	}
	if _, err := os.Stat(filepath.Join(RepoDir(s.Root), "2.md")); err != nil {
		t.Fatal(err)
	}
	if it := ok(again.Create(Item{Type: "task", Title: "Three"}, "")); it.ID != 3 {
		t.Fatalf("%+v", it)
	}
}

func TestTaskPortaElsCriterisILesNormes(t *testing.T) {
	it := Item{ID: 7, Type: "feature", Title: "X", Why: "because", Criteria: []Criterion{{Text: "a"}, {Text: "b", Done: true}}}
	task := Task(it)
	for _, want := range []string{"#7", "1. [ ] a", "2. [x] b", "in_review", "Do not mark it done"} {
		if !strings.Contains(task, want) {
			t.Errorf("falta %q a:\n%s", want, task)
		}
	}
	if got := Slug(Item{ID: 7, Title: "Export the monthly report as CSV!"}); got != "b7-export-the-monthly-repor" {
		t.Errorf("slug: %s", got)
	}
}

// tascaARevisio deixa la tasca 1 a revisió (amb on s'hi ha treballat) i
// espera l'avís al revisor.
func tascaARevisio(t *testing.T, s *Store, avisos chan Item) {
	t.Helper()
	ok := deu(t)
	ok(s.Create(Item{Type: "task", Title: "T", Why: "w", Criteria: []Criterion{{Text: "a"}, {Text: "b"}}}, ""))
	ok(s.Move(1, "ready", "", ""))
	ok(s.Move(1, "in_progress", "", ""))
	if it := ok(s.SetWork(1, "/wt/b1", "abc123")); it.Workdir != "/wt/b1" || it.Base != "abc123" {
		t.Fatalf("%+v", it)
	}
	ok(s.Check(1, 1, true, "TestA", "agent"))
	ok(s.Check(1, 2, true, "TestB", "agent"))
	ok(s.Move(1, "in_review", "", "agent"))
	if got := <-avisos; got.ID != 1 {
		t.Fatalf("avís: %+v", got)
	}
}

func TestRevisio(t *testing.T) {
	s := nouProjecte(t)
	ok := deu(t)
	avisos := make(chan Item, 4)
	OnInReview = func(root string, it Item) { avisos <- it }
	t.Cleanup(func() { OnInReview = nil })
	tascaARevisio(t, s, avisos)

	// Mentre revisa, ningú no la tanca.
	ok(s.SetReview(1, Review{Status: ReviewRunning}, "reviewer"))
	refusa(t, "still running")(s.Move(1, "done", "", "you"))

	// No superada: torna a en curs i es desmarca el criteri que falla.
	it := ok(s.SetReview(1, Review{Status: ReviewNotMet, Summary: "b has no test",
		Criteria: []ReviewCriterion{{Text: "a", Status: "MET", Evidence: "TestA ok"}, {Text: "b", Status: "UNVERIFIED", Evidence: "no test found"}}}, "reviewer"))
	if it.Status != StatusInProgress || !it.Criteria[0].Done || it.Criteria[1].Done || it.Criteria[1].Evidence != "" {
		t.Fatalf("no superada: %+v", it)
	}
	if last := it.Log[len(it.Log)-1].Text; last != "in_review → in_progress: b has no test" {
		t.Fatalf("log: %q", last)
	}

	// Torna a revisió: la revisió vella s'oblida; superada, ja es pot tancar.
	ok(s.Check(1, 2, true, "TestB added", "agent"))
	if it = ok(s.Move(1, "in_review", "", "agent")); it.Review != nil {
		t.Fatal("la revisió anterior no s'ha d'arrossegar")
	}
	<-avisos
	if it = ok(s.SetReview(1, Review{Status: ReviewMet, Checks: []ReviewCheck{{Command: "make check", Passed: true}}}, "reviewer")); it.Status != StatusInReview {
		t.Fatalf("superada: %+v", it)
	}
	if !strings.Contains(it.Log[len(it.Log)-1].Text, "`make check` pass") {
		t.Fatalf("log: %+v", it.Log[len(it.Log)-1])
	}
	ok(s.Move(1, "done", "", "you"))

	// I sobreviu a l'anada i tornada pel format del repositori.
	back, err := ParseMarkdown(Markdown(it))
	if err != nil || back.Review == nil || back.Review.Status != ReviewMet || back.Workdir != "/wt/b1" || len(back.Review.Checks) != 1 {
		t.Fatalf("%v %+v", err, back.Review)
	}
}
