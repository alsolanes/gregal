package jobs

import (
	"path/filepath"
	"testing"
	"time"
)

// TestNextDueEvent: el kind "event" és el cor del vent: una franja puntual
// (next_at) que el mateix vent es torna a programar quan corre (p. ex. just
// després del partit que vigila).
func TestNextDueEvent(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	at := now.Add(10 * time.Hour)
	j := Job{ID: "wind-xaloc", Kind: "event", Enabled: true, NextAt: &at}

	if got := NextDue(j, now); !got.Equal(at) {
		t.Fatalf("NextDue event = %v, want %v", got, at)
	}
	if Due(j, now) {
		t.Fatal("no pot tocar abans de la franja")
	}
	if !Due(j, at.Add(2*time.Minute)) {
		t.Fatal("ha de tocar passada la franja (franga perduda)")
	}
	// Franja coberta: el run ja l'ha atesa (el cor hi haurà posat next_at nou).
	last := at.Add(3 * time.Minute)
	done := Job{ID: "wind-xaloc", Kind: "event", Enabled: true, NextAt: &at, LastAt: &last}
	if got := NextDue(done, last); !got.IsZero() {
		t.Fatalf("franja coberta ha de ser zero, no %v", got)
	}
	if Due(done, last.Add(time.Hour)) {
		t.Fatal("una franja coberta no pot tornar a tocar")
	}
	// Sense next_at no toca mai (l'encès el dispara l'app o el manual).
	empty := Job{ID: "w", Kind: "event", Enabled: true}
	if got := NextDue(empty, now); !got.IsZero() {
		t.Fatalf("sense next_at ha de ser zero, no %v", got)
	}
	if Due(empty, now) {
		t.Fatal("sense next_at no toca")
	}
}

// TestSaveAcceptsEventKind i SetNextAt: el magatzem conserva el kind i permet
// que el cor es reprogrami sense perdre LastAt.
func TestSaveAndSetNextAt(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "jobs"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.Save(Job{ID: "wind-xaloc", Name: "Cor", Prompt: "p", Kind: "event", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Kind != "event" {
		t.Fatalf("kind event no acceptat: %q", saved.Kind)
	}
	at := time.Date(2026, 10, 21, 22, 50, 0, 0, time.UTC)
	if err := s.SetNextAt("wind-xaloc", at); err != nil {
		t.Fatal(err)
	}
	// Un Save posterior sense next_at (p. ex. canvi de nom) no l'ha de perdre.
	if _, err := s.Save(Job{ID: "wind-xaloc", Name: "Cor nou", Prompt: "p", Kind: "event", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].NextAt == nil || !list[0].NextAt.Equal(at) {
		t.Fatalf("next_at perdut: %+v", list)
	}
	// MarkRun actualitza l'execució sense tocar la programació.
	if err := s.MarkRun("wind-xaloc", at.Add(time.Minute), "ok"); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List()
	if list[0].NextAt == nil || !list[0].NextAt.Equal(at) {
		t.Fatalf("MarkRun ha esborrat next_at: %+v", list[0])
	}
	// Un job desconegut no crea res.
	if err := s.SetNextAt("inexistent", at); err == nil {
		list, _ := s.List()
		if len(list) != 1 {
			t.Fatalf("SetNextAt d'un job desconegut l'ha creat: %+v", list)
		}
	}
}
