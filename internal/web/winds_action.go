package web

import (
	"context"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
)

// windActionNotYet és la resposta que el vent dona quan la feina encara no es
// pot fer (el partit no ha acabat): es reintenta més tard.
const windActionNotYet = "NOT_YET"

// windActionSystem compon la consigna d'un recordatori amb feina: persona +
// encàrrec. A diferència del cor, aquí SEMPRE s'ha de contestar.
func windActionSystem(w companion.Wind, memory, userName, task string, last bool) string {
	var b strings.Builder
	b.WriteString(companion.WindIdentity(w))
	b.WriteString(w.Persona)
	if n := strings.TrimSpace(userName); n != "" {
		b.WriteString("\n\nWHO YOU TALK TO:\nThis message goes to one person: " + n + ". Use their name only when it helps, and never anyone else's.")
	}
	b.WriteString("\n\nSCHEDULED TASK. The user asked you earlier to do this at this exact moment, and the moment has come. Do it now, with your tools (web_search, web_fetch…), and report the result:\n\n" + task)
	b.WriteString("\n\nRules:\n- Give the actual result (the score, the price, the answer), 1-4 sentences, no preamble. Cite source URLs inline like [title](url).\n- Never invent facts. Reply in the user's language (Catalan if unsure).")
	if last {
		b.WriteString("\n- This is the last attempt: if you cannot get the result, say so plainly and what you found.")
	} else {
		b.WriteString("\n- If the thing is not available yet (the match has not finished, the price is not published), reply with exactly " + windActionNotYet + " and nothing else: you will be asked again later.")
	}
	if strings.TrimSpace(memory) != "" {
		b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n" + memory)
	}
	return b.String()
}

// executeWindAction dispara un recordatori amb feina: un torn de debò del vent
// (cerca web, eines) i el resultat entra al xat i notifica. Amb NOT_YET es
// reprograma sense parlar; passat MaxTries, diu què ha trobat.
func (s *Server) executeWindAction(ctx context.Context, job jobs.Job, w companion.Wind) (jobs.Run, error) {
	start := time.Now()
	tries := job.Tries + 1
	last := job.RetryMin <= 0 || tries >= job.MaxTries
	brief := windActionSystem(w, s.personalMemory(), s.windUserName(), job.Task, last)
	brief += windRemindBrief(w.ID) + companion.WindBrainSystem(s.windBrain(w.ID))
	brief += "\n\n" + config.LanguageDirective(s.cfg.Lang())
	res, runErr := windHeartbeatLoop(s, ctx, brief)
	out := strings.TrimSpace(res.text)
	end := time.Now()
	run := jobs.Run{JobID: job.ID, JobName: job.Name, StartedAt: start, FinishedAt: end}
	finish := func(status string) {
		run.Status = status
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, end, status)
	}
	if runErr == nil && !last && strings.HasPrefix(strings.ToUpper(out), windActionNotYet) {
		// Encara no: es torna a provar sense dir res; MarkRun cobreix aquesta
		// franja i SetNextAt n'obre la següent.
		job.Tries = tries
		_, _ = s.jobs.Save(job)
		finish("ok")
		_ = s.jobs.SetNextAt(job.ID, end.Add(time.Duration(job.RetryMin)*time.Minute))
		return run, nil
	}
	job.Enabled = false
	job.Tries = tries
	if runErr != nil || out == "" {
		msg := "no he pogut fer l'encàrrec: " + job.Task
		if runErr != nil {
			run.Error = runErr.Error()
		}
		postWindMessage(s, w, "remind", msg)
		run.Output = w.Name + ": " + msg
		_, _ = s.jobs.Save(job)
		finish("error")
		return run, runErr
	}
	postWindMessage(s, w, "remind", out)
	run.Output = w.Name + ": " + out
	_, _ = s.jobs.Save(job)
	finish("ok")
	return run, nil
}
