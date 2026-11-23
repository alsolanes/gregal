package gdocs

import (
	"errors"
	"fmt"
	"sync"
	"testing"
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
	d2, err := s.Replace(d.ID, blk.ID, "Second.", AuthorUser)
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
			if _, err := s2.Replace(d.ID, blk.ID, fmt.Sprint("nou ", i), AuthorAgent); err != nil {
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
