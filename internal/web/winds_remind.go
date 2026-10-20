package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Description: "Programa un avís: el vent dirà el text a l'usuari quan arribi el moment, encara que l'app estigui tancada. Fes servir SEMPRE el teu id de vent (l'el dona la consigna). at és el moment en RFC3339 amb zona horària; text és el missatge curt (màxim 500 caràcters). Només per a avisos reals: demanats per l'usuari o realment útils.",
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
	moment, err := time.Parse(time.RFC3339, strings.TrimSpace(at))
	if err != nil {
		return jobs.Job{}, fmt.Errorf("moment il·legible (cal RFC3339)")
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
	return s.jobs.Save(jobs.Job{
		ID: id, Name: "Recordatori " + w.Name, Prompt: msg,
		Kind: "remind", Enabled: true, Notify: true, NextAt: &moment,
	})
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
	hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: text})
	history := append([]llm.Message(nil), hist.messages...)
	hist.mu.Unlock()
	_ = saveWindHistory(s, w, history)
	go s.windLearn(w, source, text)
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
		" crida-la amb wind, at (moment en RFC3339 amb zona, futur, màxim d'aquí a 30 dies)" +
		" i text (el missatge curt que diràs). L'avís entrarà al teu xat i notificarà" +
		" l'usuari encara que l'app estigui tancada. Només avisos reals; no programis" +
		" mai res sense motiu ni avisis del que ja has dit en aquest torn."
}
