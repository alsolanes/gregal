package gdocs

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"gregal/internal/ooxml"
)

func obre(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDocumentsPerProjecte(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	a, b := obre(t), obre(t)
	d, err := a.Create("Plan", "# Plan\n\nFirst.", AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("un altre projecte veu el document: %v", err)
	}
	if l, _ := b.List(); len(l) != 0 {
		t.Fatalf("un altre projecte el llista: %+v", l)
	}
	if l, _ := a.List(); len(l) != 1 || l[0].Title != "Plan" || l[0].Kind != "doc" {
		t.Fatalf("%+v", l)
	}
}

func TestReplaceVersionaIAutor(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d, _ := s.Create("Plan", "# Plan\n\nFirst.", AuthorAgent)
	blk := d.Blocks[1]
	d2, err := s.Replace(d.ID, blk.ID, "Second.", AuthorUser, d.Version)
	if err != nil {
		t.Fatal(err)
	}
	got := d2.Blocks[1]
	if d2.Version <= d.Version || got.Version <= blk.Version || got.Author != AuthorUser || got.MD != "Second." {
		t.Fatalf("abans %+v, després %+v", d, d2)
	}
	if d2.Blocks[0].Author != AuthorAgent || d2.Blocks[0].Version != d.Blocks[0].Version {
		t.Fatalf("el bloc no tocat ha canviat: %+v", d2.Blocks[0])
	}
}

func TestReplacesConcurrents(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d, _ := s.Create("Plan", "a\n\nb\n\nc\n\nd", AuthorAgent)
	var wg sync.WaitGroup
	for i, blk := range d.Blocks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Cada escriptor obre el seu Store, com el web i l'eina.
			s2 := &Store{Root: s.Root, Dir: s.Dir}
			if _, err := s2.Replace(d.ID, blk.ID, fmt.Sprint("nou ", i), AuthorAgent, d.Version); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	d2, _ := s.Get(d.ID)
	for i, blk := range d2.Blocks {
		if blk.MD != fmt.Sprint("nou ", i) {
			t.Fatalf("s'ha perdut un canvi: %+v", d2.Blocks)
		}
	}
	if d2.Version != d.Version+4 {
		t.Fatalf("versió %d, volia %d", d2.Version, d.Version+4)
	}
}

func TestSplit(t *testing.T) {
	got := Split("# Title\nIntro line\n\n- a\n- b\n\n```\ncode\n\nmore\n```\n## Next")
	want := []string{"# Title", "Intro line", "- a\n- b", "```\ncode\n\nmore\n```", "## Next"}
	if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
		t.Fatalf("%q", got)
	}
}

func TestDeleteDeixaLapida(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d, _ := s.Create("Plan", "a\n\nb", AuthorAgent)
	d2, err := s.Delete(d.ID, d.Blocks[1].ID, AuthorUser, d.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.Deleted) != 1 {
		t.Fatalf("sense làpida: %+v", d2)
	}
	if l := d2.Deleted[0]; l.ID != d.Blocks[1].ID || l.Author != AuthorUser || l.Version != d2.Version || l.Since != d.Version {
		t.Fatalf("làpida %+v", l)
	}
}

func TestWatchDeletePortaAutor(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d, _ := s.Create("Plan", "a\n\nb", AuthorAgent)
	ch, stop := s.Watch(d.ID)
	defer stop()
	if _, err := s.Delete(d.ID, d.Blocks[1].ID, AuthorUser, d.Version); err != nil {
		t.Fatal(err)
	}
	c := <-ch
	if c.Op != "delete" || c.Block.ID != d.Blocks[1].ID || c.Author != AuthorUser {
		t.Fatalf("canvi %+v", c)
	}
}

func TestWatchInsertIDeleteAlMig(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d, _ := s.Create("Plan", "a\n\nb\n\nc", AuthorAgent)
	ch, stop := s.Watch(d.ID)
	defer stop()
	d2, err := s.Insert(d.ID, d.Blocks[0].ID, "x\n\ny", AuthorUser)
	if err != nil {
		t.Fatal(err)
	}
	x, y := d2.Blocks[1], d2.Blocks[2]
	c1, c2 := <-ch, <-ch
	if c1.Op != "insert" || c1.Block.ID != x.ID || c1.After != d.Blocks[0].ID || c1.Version != d2.Version ||
		c2.Op != "insert" || c2.Block.ID != y.ID || c2.After != x.ID || c2.Version != d2.Version {
		t.Fatalf("insercions %+v %+v", c1, c2)
	}
	if _, err := s.Delete(d.ID, x.ID, AuthorUser, d2.Version); err != nil {
		t.Fatal(err)
	}
	c := <-ch
	if c.Op != "delete" || c.Block.ID != x.ID {
		t.Fatalf("esborrat %+v", c)
	}
	select {
	case extra := <-ch:
		t.Fatalf("canvi de més: %+v", extra)
	default:
	}
}

func TestSeenPerSessio(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	if err := s.MarkSeen("s1", "doc1", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkSeen("s1", "doc1", 2); err != nil {
		t.Fatal(err)
	}
	if m, err := s.Seen("s1"); err != nil || m["doc1"] != 3 {
		t.Fatalf("%v %v", m, err)
	}
	if m, _ := s.Seen("s2"); len(m) != 0 {
		t.Fatalf("una altra sessió ho veu: %v", m)
	}
	if l, _ := s.List(); len(l) != 0 {
		t.Fatalf("el registre surt com a document: %+v", l)
	}
}

// deck fa una presentació amb una portada i una diapositiva «Fases».
func deck(t *testing.T, s *Store) Doc {
	t.Helper()
	d, err := s.CreateDeck("Llançament")
	if err != nil || d.Kind != KindDeck || len(d.Blocks) != 0 {
		t.Fatalf("%+v %v", d, err)
	}
	if d, err = s.InsertSlide(d.ID, "", ooxml.SlideSpec{Layout: "title", Title: "Llançament"}, AuthorAgent); err != nil {
		t.Fatal(err)
	}
	if d, err = s.InsertSlide(d.ID, d.Blocks[0].ID, ooxml.SlideSpec{Layout: "content", Title: "Fases", Bullets: []string{"Beta", "Llançament"}}, AuthorAgent); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDeckUnaDiapositivaPerBloc(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d := deck(t, s)
	fases := d.Blocks[1]
	ch, stop := s.Watch(d.ID)
	defer stop()
	d2, err := s.ReplaceSlide(d.ID, fases.ID, ooxml.SlideSpec{Layout: "two_columns", Title: "Fases", Left: "Beta", Right: "Llançament"}, AuthorUser, fases.Version)
	if err != nil {
		t.Fatal(err)
	}
	got := d2.Blocks[1]
	if got.Slide == nil || got.Slide.Layout != "two_columns" || got.Slide.Right != "Llançament" || got.Author != AuthorUser || got.Version != d2.Version || got.Since != fases.Since {
		t.Fatalf("diapositiva %+v", got)
	}
	if c := <-ch; c.Op != "replace" || c.Block.Slide == nil || c.Block.Slide.Left != "Beta" {
		t.Fatalf("canvi %+v", c)
	}
	if again, _ := s.Get(d.ID); again.Blocks[0].Slide.Title != "Llançament" || again.Blocks[1].Slide.Layout != "two_columns" {
		t.Fatalf("no s'ha desat: %+v", again.Blocks)
	}
}

// L'agent no trepitja una diapositiva que la persona ha canviat després.
func TestDeckConflicte(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d := deck(t, s)
	fases := d.Blocks[1]
	if _, err := s.ReplaceSlide(d.ID, fases.ID, ooxml.SlideSpec{Layout: "section", Title: "Fases"}, AuthorUser, fases.Version); err != nil {
		t.Fatal(err)
	}
	var c *ConflictError
	if _, err := s.ReplaceSlide(d.ID, fases.ID, ooxml.SlideSpec{Layout: "content"}, AuthorAgent, fases.Version); !errors.As(err, &c) {
		t.Fatalf("sense conflicte: %v", err)
	}
}

func TestDeckRefusaLayoutIContingutDAltreTipus(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := obre(t)
	d := deck(t, s)
	if _, err := s.ReplaceSlide(d.ID, d.Blocks[1].ID, ooxml.SlideSpec{Layout: "diagonal", Title: "x"}, AuthorAgent, d.Version); err == nil {
		t.Fatal("layout invàlid acceptat")
	}
	if _, err := s.InsertSlide(d.ID, "", ooxml.SlideSpec{Title: "sense layout"}, AuthorAgent); err == nil {
		t.Fatal("diapositiva sense layout acceptada")
	}
	if _, err := s.Insert(d.ID, "", "# text", AuthorAgent); err == nil {
		t.Fatal("markdown a una presentació")
	}
	text, _ := s.Create("Plan", "a", AuthorAgent)
	if _, err := s.InsertSlide(text.ID, "", ooxml.SlideSpec{Layout: "title"}, AuthorAgent); err == nil {
		t.Fatal("diapositiva a un document de text")
	}
}
