package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
)

// «Prep»: abans d'una reunió del calendari connectat, un torn de només lectura
// (web, calendari, correu, memòria) la prepara i la deixa a «Ara toca», com
// fan Pulse, Limitless o Duet. Una feina cada mitja hora mira els events que
// comencen d'aquí a 45-120 minuts: la finestra és més ampla que l'interval,
// així que cap reunió no s'escapa. Com a molt una targeta per event i tres per
// dia; un «No m'interessa» atura la preparació dels events amb el mateix títol.

const (
	avuiPrepSource = "prep"
	prepJobPrefix  = "avui-prep-"
	prepMaxPerDay  = 3
	prepLeadMin    = 45 * time.Minute
	prepLeadMax    = 120 * time.Minute
)

func prepJobID(owner string) string {
	if owner == "" {
		owner = "me"
	}
	return prepJobPrefix + strings.NewReplacer("/", "-", " ", "-").Replace(owner)
}

func isPrepJob(id string) bool { return strings.HasPrefix(id, prepJobPrefix) }

// ensurePrepJob crea la feina el primer cop que hi ha un calendari connectat.
// Si la persona l'apaga, es queda apagada.
func (s *Server) ensurePrepJob() error {
	id := prepJobID(s.localOwner())
	if s.jobs == nil || s.jobs.Has(id) {
		return nil
	}
	w, ok := s.ensureGregalWind()
	if !ok {
		return errors.New("the wind for meeting prep is not available")
	}
	_, err := s.jobs.Save(jobs.Job{
		ID: id, Name: s.tr("Preparo les reunions", "Meeting prep"), Prompt: "Prepare the user's next meetings",
		Kind: "interval", IntervalH: 0.5, Enabled: true, Wind: w.ID, WindOwner: s.localOwner(),
	})
	return err
}

// prepLoop és el torn de només lectura; els tests el substitueixen.
var prepLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, 8, p, role)
}

func (s *Server) prepBrief(w companion.Wind, e calEvent, title string, now time.Time) string {
	where := ""
	if l := oneLine(e.Location, 120); l != "" {
		where = " at " + l
	}
	return companion.WindIdentity(w) +
		fmt.Sprintf("\n\nTHE USER HAS A MEETING SOON: «%s», %s%s (calendar %s).\nIt is now %s (%s).\n",
			title, e.Start.In(now.Location()).Format("Monday 2 January, 15:04"), where, e.Calendar,
			now.Format("Monday 2 January 2006, 15:04"), now.Location().String()) +
		"Prepare them in a few short lines: what it is about and with whom, what the latest mails with those people say, open questions and what to bring. " +
		"Look at the calendar, the mail and your memory; search the web only if it helps (the company or person they meet). " +
		"Only facts you read; never invent. Do not send, write or change anything. Answer with only the prep, no greeting." +
		"\n\n" + config.LanguageDirective(s.cfg.Lang())
}

// prepEventTitle treu l'event del títol d'una targeta «Prep: <títol> · HH:MM».
func prepEventTitle(c avuiCard) string {
	t := strings.TrimPrefix(c.Title, "Prep: ")
	if i := strings.LastIndex(t, " · "); i >= 0 {
		t = t[:i]
	}
	return strings.ToLower(t)
}

// executePrep és el pas de la feina de preparació.
func (s *Server) executePrep(ctx context.Context, job jobs.Job) (jobs.Run, error) {
	start := time.Now()
	run := jobs.Run{Wind: job.Wind, WindOwner: job.WindOwner, JobID: job.ID, JobName: job.Name, StartedAt: start}
	finish := func(status, output, errMsg string) (jobs.Run, error) {
		run.FinishedAt, run.Status, run.Output, run.Error = time.Now(), status, output, errMsg
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, run.FinishedAt, status)
		if errMsg != "" {
			return run, errors.New(errMsg)
		}
		return run, nil
	}
	if len(s.calendarSecrets()) == 0 {
		return finish("ok", "", "")
	}
	w, ok := s.winds.ByID(job.Wind)
	if !ok {
		if w, ok = s.ensureGregalWind(); !ok {
			return finish("error", "", "the wind for meeting prep is not available")
		}
	}
	loc := s.loadConnections().zone()
	now := start.In(loc)
	evs, problems := s.calendarEvents(ctx, start, start.Add(prepLeadMax))

	avuiMu.ReadLock()
	list := loadAvui(s)
	avuiMu.ReadUnlock()
	made, muted, today := map[string]bool{}, map[string]bool{}, 0
	for _, c := range list {
		if c.Source != avuiPrepSource {
			continue
		}
		// La mateixa reunió només pot tornar a sortir dins de la finestra.
		if start.Sub(time.Unix(c.Created, 0)) < 12*time.Hour {
			made[c.Title] = true
		}
		if c.Feedback == "notuseful" {
			muted[prepEventTitle(c)] = true
		}
		if c.Created >= startOfToday(s).Unix() {
			today++
		}
	}
	var titles []string
	for _, e := range evs {
		name := oneLine(e.Summary, 90)
		lead := e.Start.Sub(start)
		title := "Prep: " + name + " · " + e.Start.In(loc).Format("15:04")
		if e.AllDay || name == "" || lead < prepLeadMin || lead > prepLeadMax || made[title] || muted[strings.ToLower(name)] {
			continue
		}
		if today >= prepMaxPerDay {
			break
		}
		res, err := prepLoop(s, ctx, s.prepBrief(w, e, name, now))
		if err != nil {
			return finish("error", strings.Join(titles, "\n"), friendlyRunError(err.Error()))
		}
		body := strings.TrimSpace(res.text)
		if body == "" {
			continue
		}
		if err := s.prepCard(w, job.ID, name, title, body); err != nil {
			return finish("error", strings.Join(titles, "\n"), err.Error())
		}
		made[title] = true
		today++
		titles = append(titles, title)
	}
	if len(problems) > 0 && len(titles) == 0 {
		return finish("error", "", strings.Join(problems, "; "))
	}
	return finish("ok", strings.Join(titles, "\n"), "")
}

// prepCard deixa la preparació a «Ara toca» i el vent ho diu al seu fil; en
// hores tranquil·les (o en mode tranquil) espera a la safata sense sonar.
func (s *Server) prepCard(w companion.Wind, job, name, title, body string) error {
	st := s.avuiSettings()
	now := time.Now()
	quiet := inQuietHours(now.In(s.loadConnections().zone()), st.QuietFrom, st.QuietTo) || st.Level == "calm"
	c := avuiCard{
		ID: avuiID(), Wind: w.ID, WindName: w.Name, Source: avuiPrepSource, Job: job, Section: avuiNow,
		Title: title, Body: body,
		Actions: []avuiAction{{Label: s.tr("Prepara-la amb mi", "Prepare it with me"),
			Prompt: s.tr("Ajuda'm a preparar aquesta reunió: ", "Help me prepare this meeting: ") + name}},
		State: avuiOpen, Held: quiet, Created: now.Unix(), Updated: now.Unix(),
	}
	avuiMu.Lock()
	err := saveAvui(s, append(loadAvui(s), c))
	avuiMu.Unlock()
	if err != nil {
		return err
	}
	postWindMessageLinked(s, w, avuiPrepSource, job, title+"\n\n"+body, windMessageLinks{Hold: quiet})
	return nil
}
