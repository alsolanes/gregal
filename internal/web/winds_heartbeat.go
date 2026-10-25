package web

import (
	"context"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
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
func windHeartbeatSystem(w companion.Wind, memory, userName string) string {
	var b strings.Builder
	b.WriteString(w.Persona)
	if n := strings.TrimSpace(userName); n != "" {
		b.WriteString("\n\nWHO YOU TALK TO:\nThis message goes to one person: " + n + ". Use their name only when it helps, and never anyone else's.")
	} else {
		b.WriteString("\n\nWHO YOU TALK TO:\nThis message goes to one person whose name you do NOT know. If a name would help, ask them; NEVER invent a name and NEVER borrow another wind's name.")
	}
	b.WriteString("\n\nYOU ARE ON WATCH. Your job (" + w.Role + ") is to check your topic with web_search and web_fetch. Rules:\n- Search the web for news about your topic. Prefer recent sources (last 24-48h).\n- Speak ONLY if you find something genuinely worth telling the user: real news, a release, a price drop, something actionable for them.\n- If nothing deserves their attention, reply with exactly: (res)\n- Never invent news. Cite source URLs inline like [title](url).\n- You can also fetch an image with web_images (album cover, product photo, diagram) and embed it as ![title](image-url) on its own line — only when it genuinely helps, never to decorate, and never invent an image URL.\n- Reply in the user's language (Catalan if unsure). 1-4 sentences, no preamble.\n- ALWAYS end your reply with a NEXT line: when you should look again. Find the next scheduled event of your topic (match, release, sale…) and set NEXT 30 minutes after it ends; if you cannot find one, 24 hours from now. Format the line exactly as: NEXT: 2026-10-21T19:30:00Z (RFC 3339, UTC).")
	if strings.TrimSpace(memory) != "" {
		b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n")
		b.WriteString(memory)
	}
	return b.String()
}

// windHeartbeatLoop és la via de cerca del cor; els tests la substitueixen.
// Com la resta de veus del vent, amb el model per defecte dels vents.
var windHeartbeatLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, 8, p, role)
}

// windNextAt extreu el moment que el vent ha programat per tornar a mirar:
// una línia "NEXT: <RFC 3339>" (UTC o amb zona). Sense línia o amb una data
// il·legible, false — qui crida hi posa el seu propi valor per defecte.
func windNextAt(reply string) (time.Time, bool) {
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`\"' "))
		if len(line) < 5 || !strings.EqualFold(line[:5], "NEXT:") {
			continue
		}
		rest := strings.TrimSpace(line[5:])
		// Acceptem també un espai extra i cometes d'eco del model.
		rest = strings.Trim(rest, "`\"' ")
		if at, err := time.Parse(time.RFC3339, rest); err == nil {
			if at.Year() > 2000 {
				return at, true
			}
		}
	}
	return time.Time{}, false
}

// windStripNext treu les línies NEXT del text que entra al xat del vent: la
// programació no és parla.
func windStripNext(reply string) string {
	lines := strings.Split(reply, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "NEXT:") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// executeWindHeartbeat és el pas del cor: cerca + (si cal) missatge al xat.
func (s *Server) executeWindHeartbeat(ctx context.Context, job jobs.Job, queued runs.Run, w companion.Wind) (jobs.Run, error) {
	start := time.Now()
	brief := windHeartbeatSystem(w, s.personalMemory(), s.usuari()) + windRemindBrief(w.ID)
	brief += companion.WindBrainSystem(s.windBrain(w.ID))
	brief += "\n\nTopic to watch: " + w.Role
	brief += "\n\n" + config.LanguageDirective(s.cfg.Lang())
	res, runErr := windHeartbeatLoop(s, ctx, brief)
	out := strings.TrimSpace(res.text)
	// El cor es reprograma: la línia "NEXT: <RFC3339>" del final és el moment
	// que el mateix vent ha triat per tornar a mirar (p. ex. just després del
	// partit). Sense línia vàlida, +24 h: mai es queda mut indefinidament.
	nextAt := time.Now().Add(24 * time.Hour)
	if at, ok := windNextAt(out); ok {
		nextAt = at
	}
	if s.jobs != nil {
		_ = s.jobs.SetNextAt(job.ID, nextAt)
	}

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
	// La línia NEXT és programació, no parla: no entra al xat.
	out = windStripNext(out)
	postWindMessage(s, w, "heartbeat", out)

	run.Status = "ok"
	// La sortida del job ÉS el missatge del vent: la notificació el porta.
	run.Output = w.Name + ": " + out
	_ = s.jobs.AddRun(run)
	_ = s.jobs.MarkRun(job.ID, end, "ok")
	return run, nil
}
