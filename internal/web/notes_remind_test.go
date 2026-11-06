package web

import (
	"gregal/internal/jobs"
	"strings"
	"testing"
	"time"
)

func noteReminderJobs(t *testing.T, s *Server) map[string]time.Time {
	t.Helper()
	list, err := s.jobs.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]time.Time{}
	for _, j := range list {
		if strings.HasPrefix(j.ID, remindJobPrefix) && j.Enabled && j.NextAt != nil {
			out[j.Prompt] = *j.NextAt
		}
	}
	return out
}

// Una tasca amb data i hora avisa a aquella hora; sense hora, a les 09:00.
// Marcar-la treu l'avís; la privada i la «sense avisos» no n'envien.
func TestNoteRemindersFollowTasks(t *testing.T) {
	s := rutinaTestServer(t)
	day := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	far := time.Now().AddDate(0, 0, 45).Format("2006-01-02")
	n := notesSave(t, s, map[string]any{"title": "Reunió", "body": "- [ ] envia el pressupost @" + day + " 16:45\n- [ ] truca a la Marta @" + day + "\n- [ ] més endavant @" + far + "\n- [ ] sense data"})
	notesSave(t, s, map[string]any{"title": "Secret", "body": "- [ ] amagada @" + day, "private": true})
	notesSave(t, s, map[string]any{"title": "Callada", "body": "- [ ] callada @" + day, "no_remind": true})

	got := noteReminderJobs(t, s)
	if len(got) != 2 {
		t.Fatalf("reminders: %v", got)
	}
	var withTime, morning time.Time
	for text, at := range got {
		if !strings.Contains(text, "Reunió") || !strings.HasPrefix(text, noteRemindPrefix) {
			t.Fatalf("text %q", text)
		}
		if strings.Contains(text, "pressupost") {
			withTime = at
		} else {
			morning = at
		}
	}
	if withTime.Hour() != 16 || withTime.Minute() != 45 || morning.Hour() != 9 || morning.Format("2006-01-02") != day {
		t.Fatalf("times %v %v", withTime, morning)
	}
	// L'agenda diu quines tasques tenen avís.
	list := notesGet(t, s, "")
	_ = list
	items := s.withRemindAt(noteAgenda(mustNotes(t, s), 0))
	marked := 0
	for _, it := range items {
		if it.RemindAt != "" {
			marked++
		}
	}
	if marked != 2 {
		t.Fatalf("remind_at marked %d", marked)
	}

	// Marcar-la com a feta treu l'avís; tornar a desar no en duplica cap.
	if _, err := s.saveNote(noteInput{ID: n.ID, Body: ptr("- [x] envia el pressupost @" + day + " 16:45\n- [ ] truca a la Marta @" + day + "\n- [ ] més endavant @" + far)}, ""); err != nil {
		t.Fatal(err)
	}
	if got := noteReminderJobs(t, s); len(got) != 1 {
		t.Fatalf("after tick: %v", got)
	}
	// Si l'usuari esborra l'avís a Automatitzacions, no torna.
	for _, j := range mustJobs(t, s) {
		if strings.HasPrefix(j.ID, remindJobPrefix) {
			_ = s.jobs.Delete(j.ID)
		}
	}
	s.syncNoteReminders()
	if got := noteReminderJobs(t, s); len(got) != 0 {
		t.Fatalf("recreated: %v", got)
	}
	// Esborrar la nota no deixa cap avís penjat.
	notesSave(t, s, map[string]any{"title": "Nova", "body": "- [ ] una altra @" + day + " 10:00"})
	if len(noteReminderJobs(t, s)) != 1 {
		t.Fatal("new task without reminder")
	}
	for _, nn := range mustNotes(t, s) {
		if nn.Title == "Nova" {
			if err := s.deleteNote(nn.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := noteReminderJobs(t, s); len(got) != 0 {
		t.Fatalf("after delete: %v", got)
	}
}

func ptr[T any](v T) *T { return &v }

func mustNotes(t *testing.T, s *Server) []Note {
	t.Helper()
	notesMu.Lock()
	defer notesMu.Unlock()
	all, err := s.loadNotesLocked()
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func mustJobs(t *testing.T, s *Server) []jobs.Job {
	t.Helper()
	l, err := s.jobs.List()
	if err != nil {
		t.Fatal(err)
	}
	return l
}
