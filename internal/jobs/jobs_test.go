package jobs

import (
	"testing"
	"time"
)

func TestDailyDue(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, loc) // diumenge 10:00
	j := Job{ID: "a", Name: "n", Kind: "daily", Time: "08:00", Enabled: true}
	if !Due(j, now) {
		t.Fatalf("08:00 amb ara=10:00 i sense execucions hauria de tocar")
	}
	slot := NextDue(j, now)
	if slot.Hour() != 8 || slot.Day() != 13 {
		t.Fatalf("franja esperada avui 08:00, tinc %v", slot)
	}
	// Executat a les 08:05 → la següent és demà.
	last := time.Date(2026, 9, 13, 8, 5, 0, 0, loc)
	j.LastAt = &last
	if Due(j, now) {
		t.Fatalf("ja coberta, no hauria de tocar")
	}
	next := NextDue(j, now)
	if next.Day() != 14 || next.Hour() != 8 {
		t.Fatalf("següent esperada demà 08:00, tinc %v", next)
	}
	// Hora futura sense execucions: catch-up immediat en crear-lo.
	j2 := Job{ID: "b", Name: "n", Kind: "daily", Time: "18:00", Enabled: true}
	if !Due(j2, now) {
		t.Fatalf("nou sense execucions hauria de fer catch-up")
	}
	// Un cop executat, la següent és avui a les 18:00.
	j2.LastAt = &now
	if Due(j2, now) {
		t.Fatalf("tot just executat, no hauria de tocar")
	}
	next2 := NextDue(j2, now)
	if next2.Day() != 13 || next2.Hour() != 18 {
		t.Fatalf("següent esperada avui 18:00, tinc %v", next2)
	}
	// Deshabilitat mai toca.
	j3 := Job{ID: "c", Name: "n", Kind: "daily", Time: "08:00"}
	if Due(j3, now) {
		t.Fatalf("deshabilitat no toca mai")
	}
	// Hora invàlida no peta.
	j4 := Job{ID: "d", Name: "n", Kind: "daily", Time: "25:99", Enabled: true}
	if Due(j4, now) {
		t.Fatalf("hora invàlida no toca")
	}
}

func TestIntervalDue(t *testing.T) {
	now := time.Now()
	j := Job{ID: "a", Name: "n", Kind: "interval", IntervalH: 6, Enabled: true}
	if !Due(j, now) {
		t.Fatalf("sense execucions hauria de tocar de seguida")
	}
	last := now.Add(-7 * time.Hour)
	j.LastAt = &last
	if !Due(j, now) {
		t.Fatalf("fa 7h amb interval 6h hauria de tocar")
	}
	recent := now.Add(-1 * time.Hour)
	j.LastAt = &recent
	if Due(j, now) {
		t.Fatalf("fa 1h amb interval 6h no hauria de tocar")
	}
	j.IntervalH = 0
	if Due(j, now) {
		t.Fatalf("interval 0 no toca")
	}
}

func TestStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.Save(Job{Name: "Notícies", Prompt: "fes butlletí", Kind: "daily", Time: "08:00", Enabled: true, Notify: true})
	if err != nil {
		t.Fatal(err)
	}
	if j.ID == "" {
		t.Fatalf("cal ID generat")
	}
	jobs, err := st.List()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("llistat: %v %v", jobs, err)
	}
	now := time.Now()
	if err := st.MarkRun(j.ID, now, "ok"); err != nil {
		t.Fatal(err)
	}
	run := Run{JobID: j.ID, JobName: j.Name, StartedAt: now, FinishedAt: now, Status: "ok", Output: "# Hola"}
	if err := st.AddRun(run); err != nil {
		t.Fatal(err)
	}
	runs, err := st.Runs("", 10)
	if err != nil || len(runs) != 1 || runs[0].Output != "# Hola" {
		t.Fatalf("runs: %v %v", runs, err)
	}
	if err := st.Delete(j.ID); err != nil {
		t.Fatal(err)
	}
	jobs, _ = st.List()
	if len(jobs) != 0 {
		t.Fatalf("hauria d'estar esborrat")
	}
	runs, _ = st.Runs(j.ID, 10)
	if len(runs) != 1 {
		t.Fatalf("les execucions es conserven")
	}
}
