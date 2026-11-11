package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// Recordatoris deterministes (B8/A3): crear, editar, ajornar, pausar i
// cancel·lar treballen sempre sobre el mateix id de job. Disparar-se no crida
// cap model: el text es desa en crear-lo i executeWindRemind el publica tal
// qual. Les dates les entén el servidor en la zona de l'usuari (tz IANA que
// envia l'app) o com a RFC3339 amb desplaçament.
//
//	POST /api/winds/remind/edit {id, op, at?, text?, minutes?, tz?}
//	  op: edit (text i/o hora) · snooze (d'aquí a N minuts) · reschedule (at)
//	      pause · resume · cancel
//	→ {ok, id, message, item?}   (item absent si s'ha cancel·lat)

// remindEditReq és una operació sobre un recordatori existent.
type remindEditReq struct {
	ID      string `json:"id"`
	Op      string `json:"op"`
	At      string `json:"at"`
	Text    string `json:"text"`
	Minutes int    `json:"minutes"`
	TZ      string `json:"tz"`
}

// remindSnoozeMax: un ajornament és curt; per a més, una hora nova.
const remindSnoozeMax = 7 * 24 * 60

// remindNow és l'hora d'ara a la zona de l'usuari (tz buida o desconeguda =
// la del servidor). parseRemindAt fa servir la zona de now per a «09:00».
func remindNow(tz string) time.Time {
	now := time.Now()
	if tz = strings.TrimSpace(tz); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return now.In(loc)
		}
	}
	return now
}

// remindFind torna el job si és una automatització pròpia d'un vent
// d'aquest compte (el mateix filtre que les accions del panell). Abans l'eina
// acceptava qualsevol job visible, inclosos els de l'administrador sense vent.
func (s *Server) remindFind(id string) (jobs.Job, bool) {
	return s.ownWindJob(strings.TrimSpace(id))
}

// remindApply aplica l'operació i torna el job tal com ha quedat. L'id no
// canvia mai: l'app, la notificació i el xat continuen apuntant al mateix.
func (s *Server) remindApply(req remindEditReq) (jobs.Job, string, error) {
	if s.jobs == nil {
		return jobs.Job{}, "", fmt.Errorf("no hi ha jobs")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return jobs.Job{}, "", fmt.Errorf("id buit")
	}
	job, ok := s.remindFind(id)
	if !ok {
		return jobs.Job{}, "", fmt.Errorf("no hi ha cap recordatori amb id %s", id)
	}
	label := func(j jobs.Job) string { return strings.TrimSpace(firstNonEmpty(j.Prompt, j.Name)) }
	isRemind := strings.HasPrefix(job.ID, remindJobPrefix) || job.Kind == "remind"
	now := remindNow(req.TZ)
	setAt := func(moment time.Time) error {
		if !moment.After(time.Now()) {
			return fmt.Errorf("l'hora nova (%s) ja és passada", moment.Format("02/01 15:04"))
		}
		if moment.After(time.Now().Add(remindMaxAhead)) {
			return fmt.Errorf("més enllà de %d dies", int(remindMaxAhead.Hours()/24))
		}
		job.NextAt = &moment
		job.Enabled = true
		// Un recordatori amb feina torna a tenir tots els intents.
		job.Tries = 0
		return nil
	}
	op := strings.TrimSpace(req.Op)
	switch op {
	case "cancel":
		if err := s.jobs.Delete(id); err != nil {
			return jobs.Job{}, "", err
		}
		return job, fmt.Sprintf("Esborrat: «%s» ja no sonarà.", label(job)), nil
	case "pause":
		job.Enabled = false
		if _, err := s.jobs.Save(job); err != nil {
			return jobs.Job{}, "", err
		}
		return job, fmt.Sprintf("Pausat: «%s» no sonarà fins que el reprènguis.", label(job)), nil
	case "resume":
		job.Enabled = true
		if _, err := s.jobs.Save(job); err != nil {
			return jobs.Job{}, "", err
		}
		return job, fmt.Sprintf("Reactivat: «%s» torna a estar actiu.", label(job)), nil
	case "reschedule", "snooze", "edit":
	default:
		return jobs.Job{}, "", fmt.Errorf("operació desconeguda: %s (usa edit, snooze, reschedule, pause, resume o cancel)", op)
	}
	if !isRemind {
		return jobs.Job{}, "", fmt.Errorf("només els recordatoris es poden reprogramar o editar així")
	}
	switch op {
	case "snooze":
		m := req.Minutes
		if m <= 0 {
			m = 10
		}
		if m > remindSnoozeMax {
			return jobs.Job{}, "", fmt.Errorf("ajornament massa llarg: posa-hi una hora nova")
		}
		// Des d'ara, no des de l'hora prevista: un reintent dona el mateix
		// resultat i un recordatori que ja ha sonat torna a quedar pendent.
		if err := setAt(time.Now().Add(time.Duration(m) * time.Minute)); err != nil {
			return jobs.Job{}, "", err
		}
	case "reschedule", "edit":
		if at := strings.TrimSpace(req.At); at != "" {
			moment, err := parseRemindAt(at, now)
			if err != nil {
				return jobs.Job{}, "", fmt.Errorf("hora no vàlida: %v", err)
			}
			if err := setAt(moment); err != nil {
				return jobs.Job{}, "", err
			}
		} else if op == "reschedule" {
			return jobs.Job{}, "", fmt.Errorf("falta l'hora nova")
		}
		if op == "edit" {
			text := strings.TrimSpace(req.Text)
			if text == "" && strings.TrimSpace(req.At) == "" {
				return jobs.Job{}, "", fmt.Errorf("res a canviar")
			}
			if text != "" {
				if len([]rune(text)) > remindMaxText {
					return jobs.Job{}, "", fmt.Errorf("text massa llarg")
				}
				job.Prompt = text
			}
		}
	}
	saved, err := s.jobs.Save(job)
	if err != nil {
		return jobs.Job{}, "", err
	}
	when := ""
	if saved.NextAt != nil {
		when = saved.NextAt.In(now.Location()).Format("02/01 15:04")
	}
	if op == "edit" && strings.TrimSpace(req.At) == "" {
		return saved, fmt.Sprintf("Desat: «%s» sonarà %s.", label(saved), when), nil
	}
	return saved, fmt.Sprintf("Reprogramat: «%s» sonarà %s.", label(saved), when), nil
}

// handleWindsRemindEdit és la porta de l'app (Android i web) a remindApply.
func (s *Server) handleWindsRemindEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req remindEditReq
	if !decodeSmall(w, r, &req) {
		return
	}
	job, msg, err := s.remindApply(req)
	if err != nil {
		code := http.StatusBadRequest
		if strings.HasPrefix(err.Error(), "no hi ha cap recordatori") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	out := map[string]any{"ok": true, "id": job.ID, "message": msg}
	if strings.TrimSpace(req.Op) != "cancel" {
		winds := map[string]companion.Wind{}
		if s.winds != nil {
			if list, err := s.winds.List(); err == nil {
				for _, wd := range list {
					winds[wd.ID] = wd
				}
			}
		}
		if it, ok := s.automationOf(job, winds, true, time.Now()); ok {
			out["item"] = it
		}
	}
	writeJSON(w, out)
}
