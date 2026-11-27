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

// Un cor antic no porta Wind al run ni al job (només l'id wind-…), i un cor
// desat amb notify=true no pot fer que acabi al bot: un vent avisa sempre a l'app.
func TestCorsSenseVentAlRunTampocPerTelegram(t *testing.T) {
	dir := t.TempDir()
	st, err := jobs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []jobs.Job{
		{ID: "wind-xaloc", Name: "Cor de Xaloc", Kind: "event", Notify: true},
		{ID: "wind-garbi@carla", Name: "Cor de Garbí", Kind: "event", Notify: true, Wind: "garbi", WindOwner: "carla"},
		{ID: "remind-ponent-1", Name: "Recorda", Kind: "remind", Notify: true, Wind: "ponent"},
	} {
		if _, err := st.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []jobs.Run{
		{JobID: "wind-xaloc", JobName: "Cor de Xaloc"},
		{JobID: "wind-garbi@carla", JobName: "Cor de Garbí"},
		{JobID: "remind-ponent-1", JobName: "Recorda"},
		{JobID: "wind-esborrat", JobName: "Cor esborrat"},
	} {
		if notifyForRun(st, r) {
			t.Errorf("%s és d'un vent: s'ha de quedar dins l'app", r.JobID)
		}
	}
}
