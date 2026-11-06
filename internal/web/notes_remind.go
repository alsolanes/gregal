package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// Avisos automàtics de les notes: cada tasca oberta amb data es converteix
// en un recordatori de vent (un job remind- de sempre, que surt a
// Automatitzacions i arriba com a notificació). Amb hora, a aquella hora;
// només amb dia, a les 09:00 d'aquell dia. Qui avisa és el vent que va
// crear la nota o, si no n'hi ha, el primer vent de l'usuari.
//
// L'estat (notes/.reminders.json) recorda quines tasques ja tenen avís:
// - una tasca marcada, esborrada o canviada de data perd el seu avís
//   pendent (i en rep un de nou si encara toca);
// - un avís que l'usuari esborra a mà no es torna a crear;
// - les notes privades o amb «sense avisos» no n'envien cap.
// Els recordatoris viuen com a molt 30 dies endavant: la resta s'hi
// programa quan entra a la finestra (en desar o obrir les notes).

const (
	noteRemindHour   = 9 // les tasques sense hora avisen a les 09:00
	noteRemindPrefix = "📝 "
)

type noteReminder struct {
	Job string `json:"job"`
	At  string `json:"at"`
}

var noteRemindMu sync.Mutex

func (s *Server) noteRemindPath() string { return filepath.Join(s.notesDir(), ".reminders.json") }

func (s *Server) loadNoteReminders() map[string]noteReminder {
	out := map[string]noteReminder{}
	if b, err := os.ReadFile(s.noteRemindPath()); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func (s *Server) saveNoteReminders(m map[string]noteReminder) error {
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.notesDir(), 0o700); err != nil {
		return err
	}
	tmp := s.noteRemindPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.noteRemindPath())
}

// noteRemindKey: la mateixa tasca (nota, text i data) és el mateix avís.
func noteRemindKey(it agendaItem) string {
	return it.NoteID + "\x00" + it.Text + "\x00" + it.Due + " " + it.Time
}

// noteRemindMoment: quan avisa una tasca (hora local del servidor).
func noteRemindMoment(it agendaItem) (time.Time, bool) {
	day, err := time.ParseInLocation("2006-01-02", it.Due, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	h, m := noteRemindHour, 0
	if it.Time != "" {
		if t, err := time.Parse("15:04", it.Time); err == nil {
			h, m = t.Hour(), t.Minute()
		}
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, time.Local), true
}

// noteRemindWind: el vent que va escriure la nota, o el primer de l'usuari.
func (s *Server) noteRemindWind(byName map[string]companion.Wind, first companion.Wind, n string) companion.Wind {
	if w, ok := byName[foldText(n)]; ok && n != "" {
		return w
	}
	return first
}

// pendingReminder: el job encara no ha sonat (el podem esborrar sense
// treure-li res a l'usuari).
func pendingReminder(list []jobs.Job, id string) bool {
	for _, j := range list {
		if j.ID == id {
			return j.Enabled && j.LastAt == nil
		}
	}
	return false
}

// syncNoteReminders posa al dia els avisos amb el que diuen les notes.
func (s *Server) syncNoteReminders() {
	if s == nil || s.jobs == nil || s.winds == nil {
		return
	}
	noteRemindMu.Lock()
	defer noteRemindMu.Unlock()
	notesMu.Lock()
	all, err := s.loadNotesLocked()
	notesMu.Unlock()
	if err != nil {
		return
	}
	winds, _ := s.winds.List()
	if len(winds) == 0 {
		return
	}
	byName := map[string]companion.Wind{}
	for _, w := range winds {
		byName[foldText(w.Name)] = w
	}
	authors := map[string]string{}
	for _, n := range all {
		authors[n.ID] = n.By
	}
	now := time.Now()
	want := map[string]agendaItem{}
	for _, it := range noteAgenda(all, 0) {
		if it.Private || it.noRemind {
			continue
		}
		at, ok := noteRemindMoment(it)
		if !ok || !at.After(now) || at.Sub(now) > remindMaxAhead {
			continue
		}
		want[noteRemindKey(it)] = it
	}
	state := s.loadNoteReminders()
	jobList, _ := s.jobs.List()
	changed := false
	// El que ja no toca: fora (i, si encara no ha sonat, sense avís).
	for k, r := range state {
		if _, ok := want[k]; ok {
			continue
		}
		if pendingReminder(jobList, r.Job) {
			_ = s.jobs.Delete(r.Job)
		}
		delete(state, k)
		changed = true
	}
	// El que és nou: un recordatori. Si l'usuari n'havia esborrat un, la
	// clau segueix a l'estat i no es torna a crear.
	seq := 0
	for k, it := range want {
		if _, ok := state[k]; ok {
			continue
		}
		at, _ := noteRemindMoment(it)
		w := s.noteRemindWind(byName, winds[0], authors[it.NoteID])
		text := noteRemindPrefix + it.Text
		if it.NoteTitle != "" {
			text += s.tr(" (de la nota «"+it.NoteTitle+"»)", " (from your note “"+it.NoteTitle+"”)")
		}
		text = clipRunes(text, remindMaxText)
		seq++
		id := fmt.Sprintf("%s%s-%d%03d", remindJobPrefix, w.ID, now.UnixMilli(), seq)
		j, err := s.jobs.Save(jobs.Job{
			ID: id, Name: "Recordatori " + w.Name, Prompt: text,
			Kind: "remind", Enabled: true, Notify: false, NextAt: &at,
			Wind: w.ID, WindOwner: s.remindOwner(),
		})
		if err != nil {
			continue
		}
		state[k] = noteReminder{Job: j.ID, At: at.Format(time.RFC3339)}
		changed = true
	}
	if changed {
		_ = s.saveNoteReminders(state)
	}
}

// withRemindAt marca a l'agenda les tasques que ja tenen avís pendent.
func (s *Server) withRemindAt(items []agendaItem) []agendaItem {
	if s == nil || s.jobs == nil {
		return items
	}
	noteRemindMu.Lock()
	state := s.loadNoteReminders()
	noteRemindMu.Unlock()
	if len(state) == 0 {
		return items
	}
	jobList, _ := s.jobs.List()
	for i := range items {
		if r, ok := state[noteRemindKey(items[i])]; ok && pendingReminder(jobList, r.Job) {
			items[i].RemindAt = r.At
		}
	}
	return items
}
