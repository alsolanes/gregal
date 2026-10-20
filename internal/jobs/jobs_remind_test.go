package jobs

import (
	"testing"
	"time"
)

// TestNormalizeRemind: el recordatori d'un vent és un kind propi (d'una sola
// execució), no un manual genèric.
func TestNormalizeRemind(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Save(Job{ID: "remind-xaloc-1", Name: "Recordatori", Kind: "remind"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "remind" {
		t.Fatalf("kind = %q, vull remind", got.Kind)
	}
}

// TestNextDueRemind: el recordatori toca quan arriba next_at i no torna a
// tocar un cop cobert (LastAt >= NextAt): d'una sola execució.
func TestNextDueRemind(t *testing.T) {
	now := time.Now()
	at := now.Add(2 * time.Hour)
	j := Job{ID: "remind-xaloc-1", Kind: "remind", Enabled: true, NextAt: &at}
	if got := NextDue(j, now); !got.Equal(at) {
		t.Fatalf("NextDue = %v, vull %v", got, at)
	}
	if Due(j, now) {
		t.Fatal("Due abans d'hora")
	}
	done := at.Add(time.Minute)
	j.LastAt = &done
	if got := NextDue(j, done); !got.IsZero() {
		t.Fatalf("NextDue després de cobert = %v, vull zero", got)
	}
	if Due(j, done) {
		t.Fatal("Due després de cobert: es repetiria")
	}
}
