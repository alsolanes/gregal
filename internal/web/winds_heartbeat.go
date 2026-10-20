package web

import (
	"context"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/runs"
)

// El cor d'un vent: la feina proactiva programada. Cada vent pot vigilar un
// tema ("música nova", "preus de GPU", "notícies de vela…"); el seu job
// busca novetats al web i, si troba algo que val la pena, li ho diu a
// l'usuari escrivint al seu xat. Si no troba res, calla: cap missatge,
// cap notificació amb soroll.

const windJobPrefix = "wind-"

// windJobID extreu l'id del vent d'un id de job (wind-tramuntana → tramuntana).
func windJobID(jobID string) (string, bool) {
	if !strings.HasPrefix(jobID, windJobPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(jobID, windJobPrefix)
	return id, id != ""
}

// WindJobID construeix l'id del job d'un vent.
func WindJobID(windID string) string { return windJobPrefix + windID }

// windHeartbeatSystem compon la consigna del cor: persona + tema a vigilar.
func windHeartbeatSystem(w companion.Wind, memory string) string {
	var b strings.Builder
	b.WriteString(w.Persona)
	b.WriteString("\n\nYOU ARE ON WATCH. Your job (" + w.Role + ") is to check your topic with web_search and web_fetch. Rules:\n- Search the web for news about your topic. Prefer recent sources (last 24-48h).\n- Speak ONLY if you find something genuinely worth telling the user: real news, a release, a price drop, something actionable for them.\n- If nothing deserves their attention, reply with exactly: (res)\n- Never invent news. Cite source URLs inline like [title](url).\n- Reply in the user's language (Catalan if unsure). 1-4 sentences, no preamble.")
	if strings.TrimSpace(memory) != "" {
		b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n")
		b.WriteString(memory)
	}
	return b.String()
}

// windHeartbeatLoop és la via de cerca del cor; els tests la substitueixen.
var windHeartbeatLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	return s.runReadOnlyLoop(ctx, brief, 8)
}

// executeWindHeartbeat és el pas del cor: cerca + (si cal) missatge al xat.
func (s *Server) executeWindHeartbeat(ctx context.Context, job jobs.Job, queued runs.Run, w companion.Wind) (jobs.Run, error) {
	start := time.Now()
	brief := windHeartbeatSystem(w, s.personalMemory())
	brief += companion.WindBrainSystem(s.windBrain(w.ID))
	brief += "\n\nTopic to watch: " + w.Role
	brief += "\n\n" + config.LanguageDirective(s.cfg.Lang())
	res, runErr := windHeartbeatLoop(s, ctx, brief)
	out := strings.TrimSpace(res.text)

	end := time.Now()
	run := jobs.Run{
		JobID: job.ID, JobName: job.Name, StartedAt: start, FinishedAt: end,
	}
	if runErr != nil {
		run.Status = "error"
		run.Error = runErr.Error()
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, "error")
		return run, runErr
	}
	// Silenci: res que valgui la pena. Job OK, sense missatge ni notificació.
	if out == "" || out == "(res)" {
		run.Status = "ok"
		run.Output = ""
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, "ok")
		return run, nil
	}
	// Hi ha algo a dir: entra al xat del vent com si hagués parlat ell.
	hist := windHistory(s, w.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: out})
	history := append([]llm.Message(nil), hist.messages...)
	hist.mu.Unlock()
	_ = saveWindHistory(s, w, history)
	// I aprèn el que ha explicat.
	go s.windLearn(w, "heartbeat", out)

	run.Status = "ok"
	// La sortida del job ÉS el missatge del vent: la notificació el porta.
	run.Output = w.Name + ": " + out
	_ = s.jobs.AddRun(run)
	_ = s.jobs.MarkRun(job.ID, end, "ok")
	return run, nil
}
