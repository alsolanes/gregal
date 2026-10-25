package telegram

import (
	"testing"

	"gregal/internal/jobs"
)

// Els runs dels vents no s'han de notificar MAI per Telegram: els vents
// avisen dins l'app (novetats), no per cap bot.
func TestRunsDeVentMaiPerTelegram(t *testing.T) {
	dir := t.TempDir()
	st, err := jobs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(jobs.Job{ID: "feina", Name: "Butlletí", Notify: true}); err != nil {
		t.Fatal(err)
	}
	// Run d'un vent (cor): mai.
	if notifyForRun(st, jobs.Run{JobID: "wind-xaloc", Wind: "w-123", JobName: "Cor de Xaloc"}) {
		t.Error("un run de vent s'ha de quedar dins l'app, mai anar al bot")
	}
	// Run normal d'un job amb notify: sí.
	if !notifyForRun(st, jobs.Run{JobID: "feina", JobName: "Butlletí"}) {
		t.Error("un job normal amb notify=true s'ha de poder notificar")
	}
	// Run d'un job esborrat (no vent): avisa igualment (com abans).
	if !notifyForRun(st, jobs.Run{JobID: "vell", JobName: "Vell"}) {
		t.Error("un job esborrat es notifica com abans")
	}
}
