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
	b.WriteString(companion.WindIdentity(w))
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

// windIsNextLine diu si una línia és la programació NEXT, amb el mateix
// criteri que windNextAt (majúscules, espais i cometes d'eco del model).
func windIsNextLine(line string) bool {
	line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`\"' "))
	return len(line) >= 5 && strings.EqualFold(line[:5], "NEXT:")
}

// windStripNext treu les línies NEXT del text que entra al xat del vent: la
// programació no és parla.
func windStripNext(reply string) string {
	lines := strings.Split(reply, "\n")
	out := lines[:0]
	for _, line := range lines {
		if windIsNextLine(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// windIsSilence diu si el que queda (ja sense NEXT) és el silenci del cor:
// buit o «(res)», tolerant cometes, punt final i majúscules.
func windIsSilence(text string) bool {
	t := strings.TrimSpace(strings.Trim(strings.TrimSpace(text), "`\"'*. "))
	return t == "" || strings.EqualFold(t, "(res)") || strings.EqualFold(t, "res")
}

// Límits de la propera mirada del cor: ni un bucle (una data passada o massa
// propera faria córrer el job sense parar) ni silenci indefinit (una data
// d'aquí a anys deixaria el vent mut).
const (
	windNextMin     = 30 * time.Minute
	windNextMax     = 14 * 24 * time.Hour
	windNextDefault = 24 * time.Hour
)

// windClampNext decideix la propera mirada a partir del que ha demanat el
// model (si n'ha demanat cap). Pura: testejable.
func windClampNext(at time.Time, ok bool, now time.Time) time.Time {
	if !ok {
		return now.Add(windNextDefault)
	}
	if at.Before(now.Add(windNextMin)) {
		return now.Add(windNextMin)
	}
	if at.After(now.Add(windNextMax)) {
		return now.Add(windNextMax)
	}
	return at
}

// executeWindHeartbeat és el pas del cor: cerca + (si cal) missatge al xat.
//
// Des de B6 el cor és una responsabilitat més (cor-<vent>): el mateix pas de
// sempre, amb pressupost de compute i d'atenció, deduplicació i el registre
// que respon «per què ho has fet?». La pausa és el job deshabilitat, com
// abans (el programador no el dispara).
func (s *Server) executeWindHeartbeat(ctx context.Context, job jobs.Job, queued runs.Run, w companion.Wind) (jobs.Run, error) {
	start := time.Now()
	stored := job
	if j, ok := s.respFindJobSafe(job.ID); ok {
		stored = j
	} else {
		stored.Enabled = true
	}
	resp := s.respEnsureHeartbeat(w, stored)
	if !s.respHeartGate(resp, start) {
		// Pressupost de compute esgotat: cap crida al model ni missatge.
		// Torna a mirar demà.
		y, m, d := start.Date()
		next := time.Date(y, m, d+1, 0, 5, 0, 0, start.Location())
		if s.jobs != nil {
			_ = s.jobs.SetNextAt(job.ID, next)
			_ = s.jobs.MarkRun(job.ID, start, "ok")
		}
		return jobs.Run{JobID: job.ID, JobName: job.Name, StartedAt: start, FinishedAt: start, Status: "ok"}, nil
	}
	brief := windHeartbeatSystem(w, s.personalMemory(), s.windUserName()) + windRemindBrief(w.ID)
	brief += companion.WindBrainSystem(s.windBrain(w.ID)) + s.windMemorySystem(w.ID)
	brief += windTradeBrief(w) + s.windCalaixBrief(w)
	brief += "\n\nTopic to watch: " + w.Role
	// El que la persona ha marcat com a útil o no a la safata «Avui».
	brief += s.avuiFeedbackBrief(w.ID)
	brief += "\n\n" + config.LanguageDirective(s.cfg.Lang())
	res, runErr := windHeartbeatLoop(s, ctx, brief)
	out := strings.TrimSpace(res.text)
	// El cor es reprograma: la línia "NEXT: <RFC3339>" del final és el moment
	// que el mateix vent ha triat per tornar a mirar (p. ex. just després del
	// partit). Sense línia vàlida, +24 h: mai es queda mut indefinidament.
	at, ok := windNextAt(out)
	nextAt := windClampNext(at, ok, time.Now())
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
	// Primer es treu la programació i després es mira si és silenci: «(res)»
	// seguit de la línia NEXT (el que demana el prompt) no és cap missatge.
	out = windStripNext(out)
	// Silenci: res que valgui la pena. Job OK, sense missatge ni notificació.
	if windIsSilence(out) {
		s.respHeartAfter(resp, "", queued.ID, end)
		run.Status = "ok"
		run.Output = ""
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, "ok")
		return run, nil
	}
	publish, hold, resultID := s.respHeartAfter(resp, out, queued.ID, end)
	run.Status = "ok"
	if !publish {
		// El mateix que ja va dir fa poc: silenci.
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, "ok")
		return run, nil
	}
	// Hi ha algo a dir: entra al xat del vent com si hagués parlat ell,
	// lligat al run i al resultat.
	links := windMessageLinks{RunID: queued.ID, Learn: true, Hold: hold}
	if resultID != "" {
		links.Refs = []string{resultID}
	}
	msgID := postWindMessageLinked(s, w, "heartbeat", "", out, links)
	s.respHeartMessage(resp, resultID, msgID)

	// La sortida del job ÉS el missatge del vent: la notificació el porta.
	// Amb el pressupost d'atenció esgotat, va al fil però no sona.
	if !hold {
		run.Output = w.Name + ": " + out
	}
	_ = s.jobs.AddRun(run)
	_ = s.jobs.MarkRun(job.ID, end, "ok")
	return run, nil
}
