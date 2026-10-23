package jobs

import (
	"testing"
	"time"
)

// TestDueRemindNotEarly: «recorda'm d'aquí a 1 minut» no pot sonar als pocs
// segons de crear-se. Toca a next_at, mai abans; un tick tardà la troba igual.
func TestDueRemindNotEarly(t *testing.T) {
	created := time.Date(2026, 10, 23, 21, 14, 30, 0, time.Local)
	at := created.Add(60 * time.Second)
	j := Job{ID: "remind-xaloc-1", Kind: "remind", Enabled: true, NextAt: &at, CreatedAt: created}
	for _, early := range []time.Duration{5 * time.Second, 7 * time.Second, 30 * time.Second, 59 * time.Second} {
		if Due(j, created.Add(early)) {
			t.Fatalf("Due a +%v amb next_at a +60s: ha sonat abans d'hora", early)
		}
	}
	if !Due(j, at) {
		t.Fatal("ha de tocar just a next_at")
	}
	if !Due(j, at.Add(29*time.Second)) {
		t.Fatal("ha de tocar si el tick arriba tard")
	}
	// Un event (el cor del vent) segueix la mateixa regla.
	ev := Job{ID: "wind-xaloc", Kind: "event", Enabled: true, NextAt: &at}
	if Due(ev, at.Add(-30*time.Second)) {
		t.Fatal("event: ha sonat abans de next_at")
	}
	if !Due(ev, at) {
		t.Fatal("event: ha de tocar a next_at")
	}
}

// TestDueDailyAtSlot: una diària de les 09:00 corre a les 09:00 o just
// després, no a les 08:59; i un cop coberta, no torna fins demà. Abans, amb
// el minut de gràcia, un tick de les 08:59:30 la disparava i, si l'execució
// acabava abans de les 09:00, la franja quedava sense cobrir i corria dues
// vegades.
func TestDueDailyAtSlot(t *testing.T) {
	loc := time.Local
	slot := time.Date(2026, 10, 23, 9, 0, 0, 0, loc)
	yesterday := slot.Add(-24*time.Hour + 20*time.Second)
	j := Job{ID: "a", Name: "n", Kind: "daily", Time: "09:00", Enabled: true, LastAt: &yesterday}

	if Due(j, slot.Add(-30*time.Second)) {
		t.Fatal("08:59:30: no ha de tocar abans de les 09:00")
	}
	if !Due(j, slot) {
		t.Fatal("09:00:00: ha de tocar")
	}
	if !Due(j, slot.Add(29*time.Second)) {
		t.Fatal("09:00:29: ha de tocar si el tick arriba tard")
	}
	ran := slot.Add(5 * time.Second)
	j.LastAt = &ran
	if Due(j, slot.Add(35*time.Second)) {
		t.Fatal("coberta a les 09:00:05: no pot tornar a tocar")
	}
	if next := NextDue(j, slot.Add(35*time.Second)); !next.Equal(slot.Add(24 * time.Hour)) {
		t.Fatalf("següent esperada demà 09:00, tinc %v", next)
	}
	if Due(j, slot.Add(24*time.Hour-30*time.Second)) {
		t.Fatal("demà 08:59:30: no ha de tocar abans d'hora")
	}
	if !Due(j, slot.Add(24*time.Hour)) {
		t.Fatal("demà 09:00: ha de tocar")
	}
}

// TestDueIntervalAtSlot: l'interval toca quan es compleix, no un minut abans.
func TestDueIntervalAtSlot(t *testing.T) {
	last := time.Date(2026, 10, 23, 6, 0, 0, 0, time.Local)
	j := Job{ID: "a", Name: "n", Kind: "interval", IntervalH: 6, Enabled: true, LastAt: &last}
	slot := last.Add(6 * time.Hour)
	if Due(j, slot.Add(-30*time.Second)) {
		t.Fatal("30 s abans de complir l'interval no ha de tocar")
	}
	if !Due(j, slot) {
		t.Fatal("complert l'interval ha de tocar")
	}
}
