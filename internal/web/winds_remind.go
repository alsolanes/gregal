package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/llm"
)

// remindJobPrefix identifica els recordatoris: els dispatcha
// executeWindRemind (diuen el text i s'apaguen), no el cor.
const remindJobPrefix = "remind-"

// remindMaxText i remindMaxAhead són els límits d'un avís: curt i com a
// màxim d'aquí a 30 dies.
const remindMaxText = 500

const remindMaxAhead = 30 * 24 * time.Hour

var remindToolSpec = llm.ToolSpec{
	Name:        "remind",
	Description: "Programa un avís: el vent dirà el text a l'usuari quan arribi el moment, encara que l'app estigui tancada. Fes servir SEMPRE el teu id de vent (l'el dona la consigna). at és el moment: RFC3339 amb zona, o 21/10 09:00, o 09:00 (avui o demà), o +2h/+30m; sempre futur, màxim d'aquí a 30 dies. text és el missatge curt (màxim 500 caràcters). CRIDA L'EINA ABANS de dir a l'usuari que ho faràs: si la crida falla, digue-li què ha fallat en comptes de prometre res. Només per a avisos reals.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind": map[string]any{"type": "string"},
		"at":   map[string]any{"type": "string"},
		"text": map[string]any{"type": "string"},
	}, "required": []string{"wind", "at", "text"}},
}

// remindJobWind treu el vent d'un id (remind-<vent>-<segons>).
func remindJobWind(jobID string) (string, bool) {
	rest, ok := strings.CutPrefix(jobID, remindJobPrefix)
	if !ok {
		return "", false
	}
	i := strings.LastIndex(rest, "-")
	if i <= 0 {
		return "", false
	}
	wind, stamp := rest[:i], rest[i+1:]
	if wind == "" || stamp == "" {
		return "", false
	}
	for _, c := range stamp {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return wind, true
}

// remindEvent és una crida real a l'eina: la prova que el vent ha actuat.
type remindEvent struct {
	wind string
	at   string // "02/01 15:04"
	text string
	when time.Time
}

var remindEventsMu sync.Mutex
var remindEvents []remindEvent

func recordRemindEvent(windID string, at time.Time, text string) {
	remindEventsMu.Lock()
	defer remindEventsMu.Unlock()
	remindEvents = append(remindEvents, remindEvent{windID, at.Format("02/01 15:04"), text, time.Now()})
	// Anell curt: només cal el torn en curs.
	if len(remindEvents) > 32 {
		remindEvents = remindEvents[len(remindEvents)-32:]
	}
}

// remindActionsSince torna els avisos que aquest vent ha programat des de t0:
// el xat els mostra i l'usuari veu que l'eina s'ha cridat de debò.
func remindActionsSince(windID string, t0 time.Time) []map[string]string {
	remindEventsMu.Lock()
	defer remindEventsMu.Unlock()
	var out []map[string]string
	for _, e := range remindEvents {
		if e.wind == windID && !e.when.Before(t0) {
			out = append(out, map[string]string{"kind": "remind", "at": e.at, "text": e.text})
		}
	}
	return out
}

// parseRemindAt entén el moment en diversos formats (el model no sempre
// encerta l'RFC3339): RFC3339, "2006-01-02 15:04", "02/01/2006 15:04",
// "15:04" (avui o demà) i relatius "+30m"/"+2h". Tot en hora local.
func parseRemindAt(raw string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("moment buit")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	local := now.Location()
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "02/01/2006 15:04", "02/01 15:04"} {
		if t, err := time.ParseInLocation(layout, s, local); err == nil {
			at := t
			if layout == "02/01 15:04" {
				at = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, local)
			}
			if !at.After(now) {
				at = at.AddDate(1, 0, 0)
			}
			return at, nil
		}
	}
	if len(s) == 5 && s[2] == ':' {
		if t, err := time.ParseInLocation("15:04", s, local); err == nil {
			at := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, local)
			if !at.After(now) {
				at = at.Add(24 * time.Hour)
			}
			return at, nil
		}
	}
	if d, err := time.ParseDuration(strings.TrimPrefix(s, "+")); err == nil && strings.HasPrefix(s, "+") && d > 0 {
		return now.Add(d), nil
	}
	return time.Time{}, fmt.Errorf("moment il·legible (p. ex. 21/10 09:00 o +2h)")
}

// saveWindReminder desa l'avís d'un vent: el vent existeix, el moment és
// futur (màxim 30 dies) i el text és curt. Torna el job desat.
func saveWindReminder(s *Server, windID, at, text string) (jobs.Job, error) {
	if s.winds == nil || s.jobs == nil {
		return jobs.Job{}, fmt.Errorf("recordatoris no disponibles")
	}
	w, ok := s.winds.ByID(strings.TrimSpace(windID))
	if !ok {
		return jobs.Job{}, fmt.Errorf("vent inexistent")
	}
	moment, err := parseRemindAt(at, time.Now())
	if err != nil {
		return jobs.Job{}, err
	}
	if !moment.After(time.Now()) {
		return jobs.Job{}, fmt.Errorf("el moment ja ha passat")
	}
	if moment.Sub(time.Now()) > remindMaxAhead {
		return jobs.Job{}, fmt.Errorf("màxim d'aquí a 30 dies")
	}
	msg := strings.TrimSpace(text)
	if msg == "" {
		return jobs.Job{}, fmt.Errorf("text buit")
	}
	if len([]rune(msg)) > remindMaxText {
		return jobs.Job{}, fmt.Errorf("text massa llarg")
	}
	id := fmt.Sprintf("%s%s-%d", remindJobPrefix, w.ID, time.Now().Unix())
	j, err := s.jobs.Save(jobs.Job{
		ID: id, Name: "Recordatori " + w.Name, Prompt: msg,
		// Notify:false — l'avís arriba per notificació local de l'app
		// (novetats); el bot de Telegram hi afegiria un duplicat genèric
		// («llest, mira-ho al panell») i els vents no hi parlen.
		Kind: "remind", Enabled: true, Notify: false, NextAt: &moment,
	})
	if err != nil {
		return jobs.Job{}, err
	}
	recordRemindEvent(w.ID, moment, msg)
	return j, nil
}

// registerRemindTool exposa l'eina remind al loop (la criden els vents).
func (s *Server) registerRemindTool() {
	agent.RegisterExtraCtx(remindToolSpec, func(_ context.Context, argsJSON string) (string, error) {
		var a struct {
			Wind string `json:"wind"`
			At   string `json:"at"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", fmt.Errorf("arguments il·legibles")
		}
		j, err := saveWindReminder(s, a.Wind, a.At, a.Text)
		if err != nil {
			return "", err
		}
		return "Fet: avisaré el " + j.NextAt.Format("02/01 15:04") + ".", nil
	})
}

// postWindMessage fa parlar el vent: el text entra al seu xat com si
// l'hagués dit ell, es desa i n'aprèn. Ho comparteixen el cor i els
// recordatoris.
func postWindMessage(s *Server, w companion.Wind, source, text string) {
	hist := windHistory(s, w.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: text, Ts: time.Now().Unix()})
	history := append([]llm.Message(nil), hist.messages...)
	hist.mu.Unlock()
	_ = saveWindHistory(s, w, history)
	goBackground(func() { s.windLearn(w, source, text) })
	// Registre per a l'app (notificació local): el missatge és proactiu,
	// l'usuari pot tenir l'app en segon pla. Best-effort, mai no bloqueja.
	recordNovetat(s, w, source, text)
}

// executeWindRemind dispara un recordatori: el text entra al xat del vent,
// la sortida (notificació) el porta i el job s'apaga (una sola execució).
func (s *Server) executeWindRemind(job jobs.Job) (jobs.Run, error) {
	end := time.Now()
	fail := func(msg string) (jobs.Run, error) {
		run := jobs.Run{JobID: job.ID, JobName: job.Name, StartedAt: end, FinishedAt: end, Status: "error", Error: msg}
		job.Enabled = false
		_, _ = s.jobs.Save(job)
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, "error")
		return run, fmt.Errorf("%s", msg)
	}
	if s.winds == nil || s.jobs == nil {
		return fail("recordatoris no disponibles")
	}
	windID, ok := remindJobWind(job.ID)
	if !ok {
		return fail("id de recordatori mal format")
	}
	w, ok := s.winds.ByID(windID)
	if !ok {
		return fail("el vent ja no existeix")
	}
	text := strings.TrimSpace(job.Prompt)
	if text == "" {
		return fail("recordatori sense text")
	}
	postWindMessage(s, w, "remind", text)
	run := jobs.Run{JobID: job.ID, JobName: job.Name, StartedAt: end, FinishedAt: end, Status: "ok", Output: w.Name + ": " + text}
	job.Enabled = false
	_, _ = s.jobs.Save(job)
	_ = s.jobs.AddRun(run)
	_ = s.jobs.MarkRun(job.ID, end, "ok")
	return run, nil
}

// windRemindBrief diu al vent com programar avisos (eina remind).
func windRemindBrief(windID string) string {
	now := time.Now().Format(time.RFC3339)
	return "\n\nEl teu id de vent és \"" + windID + "\" (fes-lo servir tal qual a wind)." +
		" Ara són " + now + " (zona local)." +
		" Pots programar avisos amb l'eina remind: si l'usuari et demana que l'avisis" +
		" d'alguna cosa, o tu creus que cal avisar-lo més tard d'algun fet d'aquest torn," +
		" crida-la amb wind, at (RFC3339 amb zona, o 21/10 09:00, o 09:00, o +2h;" +
		" sempre futur, màxim d'aquí a 30 dies)" +
		" i text (el missatge curt que diràs). IMPORTANT: crida l'eina ABANS de" +
		" dir a l'usuari que ho faràs; si falla, explica-li què ha fallat i no" +
		" prometis res. L'avís entrarà al teu xat i notificarà" +
		" l'usuari encara que l'app estigui tancada. Només avisos reals; no programis" +
		" mai res sense motiu ni avisis del que ja has dit en aquest torn."
}
