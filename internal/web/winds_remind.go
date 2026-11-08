package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
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

// remindMaxTries: cops que un recordatori amb feina es reintenta si el vent
// diu NOT_YET (el partit encara no ha acabat).
const remindMaxTries = 8

var remindToolSpec = llm.ToolSpec{
	Name:        "remind",
	Description: "Programa un avís: el vent dirà el text a l'usuari quan arribi el moment, encara que l'app estigui tancada. Fes servir SEMPRE el teu id de vent (l'el dona la consigna). at és el moment: RFC3339 amb zona, o 21/10 09:00, o 09:00 (avui o demà), o +2h/+30m; sempre futur, màxim d'aquí a 30 dies. text és el missatge curt (màxim 500 caràcters). CRIDA L'EINA ABANS de dir a l'usuari que ho faràs: si la crida falla, digue-li què ha fallat en comptes de prometre res. Només per a avisos reals. Si l'usuari vol que a aquella hora FACIS alguna cosa (p. ex. «quan acabi el partit avisa'm del resultat», «mira el preu i digue'm-ho»), posa a task què has de fer (p. ex. «Busca el resultat final del Barça-Madrid i digue'l»): a l'hora faràs un torn complet amb cerca web i diràs el resultat; text és llavors un resum curt de l'encàrrec. Si pot no estar llest a l'hora (un partit que dura), posa at una mica després del final previst i retry_minutes (p. ex. 15): es reintentarà fins a 8 cops. followup=true quan és un seguiment que t'ofereixes a fer tu (ningú no te l'ha demanat): l'usuari el veu com a seguiment i el pot desfer.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind":          map[string]any{"type": "string"},
		"at":            map[string]any{"type": "string"},
		"text":          map[string]any{"type": "string"},
		"task":          map[string]any{"type": "string"},
		"retry_minutes": map[string]any{"type": "integer"},
		"followup":      map[string]any{"type": "boolean"},
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
	owner    string // compte que l'ha demanat ("" en local): dos comptes poden tenir el mateix vent
	id       string // job desat: l'app el fa servir per desfer-lo
	followup bool
	wind     string
	at       string // "02/01 15:04"
	atTime   time.Time
	text     string
	when     time.Time
}

var remindEventsMu sync.Mutex
var remindEvents []remindEvent

func recordRemindEvent(owner, jobID string, followup bool, windID string, at time.Time, text string) {
	remindEventsMu.Lock()
	defer remindEventsMu.Unlock()
	remindEvents = append(remindEvents, remindEvent{owner, jobID, followup, windID, at.Format("02/01 15:04"), at, text, time.Now()})
	// Anell curt: només cal el torn en curs.
	if len(remindEvents) > 32 {
		remindEvents = remindEvents[len(remindEvents)-32:]
	}
}

// remindActionsSince torna els avisos que aquest vent ha programat des de t0:
// el xat els mostra i l'usuari veu que l'eina s'ha cridat de debò.
// at_iso és el moment exacte (RFC3339): l'app connectada a un servidor
// hi programa la propera passada perquè l'avís no arribi tard.
func remindActionsSince(owner, windID string, t0 time.Time) []map[string]string {
	remindEventsMu.Lock()
	defer remindEventsMu.Unlock()
	var out []map[string]string
	for _, e := range remindEvents {
		if e.owner == owner && e.wind == windID && !e.when.Before(t0) {
			kind := "remind"
			if e.followup {
				kind = "followup"
			}
			out = append(out, map[string]string{"kind": kind, "id": e.id, "at": e.at, "at_iso": e.atTime.Format(time.RFC3339), "text": e.text})
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
	return saveWindReminderTask(s, windID, at, text, "", 0)
}

// followupMaxPending és quants seguiments pendents pot tenir un vent alhora:
// un assistent atent, no un que ho converteix tot en avisos.
const followupMaxPending = 3

func saveWindReminderTask(s *Server, windID, at, text, task string, retryMin int) (jobs.Job, error) {
	return saveWindReminderFull(s, windID, at, text, task, retryMin, false)
}

// saveWindReminderTask és saveWindReminder amb una feina opcional: a l'hora el
// vent fa un torn de debò (task) en comptes de dir només el text.
func saveWindReminderFull(s *Server, windID, at, text, task string, retryMin int, followup bool) (jobs.Job, error) {
	if s.winds == nil || s.jobs == nil {
		return jobs.Job{}, fmt.Errorf("recordatoris no disponibles")
	}
	w, ok := windByRef(s, strings.TrimSpace(windID))
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
	task = strings.TrimSpace(task)
	msg := strings.TrimSpace(text)
	if msg == "" {
		msg = task
	}
	if msg == "" {
		return jobs.Job{}, fmt.Errorf("text buit")
	}
	if len([]rune(task)) > 1000 {
		return jobs.Job{}, fmt.Errorf("feina massa llarga")
	}
	maxTries := 0
	if task != "" {
		maxTries = 1
		if retryMin > 0 {
			retryMin = min(max(retryMin, 5), 240)
			maxTries = remindMaxTries
		}
	}
	if len([]rune(msg)) > remindMaxText {
		return jobs.Job{}, fmt.Errorf("text massa llarg")
	}
	owner := s.remindOwner()
	if followup {
		if !w.FollowupsOn() {
			return jobs.Job{}, fmt.Errorf("l'usuari ha desactivat els seguiments d'aquest vent: no n'ofereixis")
		}
		if err := s.followupRoom(w.ID, moment); err != nil {
			return jobs.Job{}, err
		}
	}
	// ID amb nanosegons: dos remind desats en el mateix mil·lisegon (tests,
	// o dos vents a la vegada) col·lisionaven i Store.Save substituïa el
	// primer en comptes d'afegir el segon — l'usuari perdia un avís.
	id := fmt.Sprintf("%s%s-%d", remindJobPrefix, w.ID, time.Now().UnixNano())
	j, err := s.jobs.Save(jobs.Job{
		ID: id, Name: "Recordatori " + w.Name, Prompt: msg,
		// Notify:false — l'avís arriba per notificació local de l'app
		// (novetats); el bot de Telegram hi afegiria un duplicat genèric
		// («llest, mira-ho al panell») i els vents no hi parlen.
		Kind: "remind", Enabled: true, Notify: false, NextAt: &moment,
		Wind: w.ID, WindOwner: owner,
		Task: task, RetryMin: retryMin, MaxTries: maxTries, Followup: followup,
	})
	if err != nil {
		return jobs.Job{}, err
	}
	recordRemindEvent(owner, j.ID, followup, w.ID, moment, msg)
	return j, nil
}

// remindListToolSpec: l'altre costat de remind. El vent pot programar avisos
// però no podia veure els que ja hi ha: quan l'usuari pregunta «quins
// recordatoris tinc?» responia que no tenia cap eina. Llista els pendents
// del compte (tots els vents de l'usuari), amb moment, text i estat.
var remindListToolSpec = llm.ToolSpec{
	Name: "remind_list",
	Description: "Llista els recordatoris i automatitzacions pendents de l'usuari: els que qualsevol vent ha programat (avisos, seguiments, feines amb hora) i les rutines actives. Fes-la servir SEMPRE que l'usuari pregunti quins recordatoris té, si tens res pendent, o què li has de recordar; no responguis mai «no ho sé» — crida l'eina. Retorna una llista amb el moment, el text i l'estat de cada pendent. Si en vols esborrar o ajornar un, digues-ho a l'usuari amb l'id: encara no hi ha eina per fer-ho.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind": map[string]any{"type": "string", "description": "opcional: llista només els d'aquest vent (id)"},
	}, "required": []string{}},
}

// remindListItems llegeix els jobs i en fa la llista llegible per al model.
// Reutilitza automationOf (la mateixa font que el panell d'automatitzacions)
// filtrant per compte: només els recordatoris dels vents de l'usuari.
func (s *Server) remindListItems(windID string) ([]map[string]string, error) {
	if s.winds == nil || s.jobs == nil {
		return nil, fmt.Errorf("no hi ha jobs")
	}
	list, err := s.winds.List()
	if err != nil {
		return nil, err
	}
	byID := map[string]companion.Wind{}
	for _, wd := range list {
		if windID == "" || wd.ID == windID {
			byID[wd.ID] = wd
		}
	}
	all, err := s.jobs.List()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := []map[string]string{}
	for _, j := range all {
		it, ok := s.automationOf(j, byID, windID == "", now)
		if !ok {
			continue
		}
		// Només pendents del compte: el filtre d'owner el fa automationOf
		// via canSeeWindRoutine/WindOwner — mateixa font que el panell.
		row := map[string]string{
			"id": it.ID, "vent": it.WindName, "kind": it.Kind,
			"titol": it.Title,
		}
		if it.NextAt != nil {
			row["moment"] = it.NextAt.Format("02/01 15:04")
		}
		if !it.Enabled {
			row["estat"] = "pausat"
		} else {
			row["estat"] = "actiu"
		}
		if it.Detail != "" {
			row["detall"] = it.Detail
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return out, nil
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a]["moment"] < out[b]["moment"]
	})
	return out, nil
}

// remindEditToolSpec: el costat d'escriptura dels recordatoris. El vent
// podia programar i llistar avisos però no tocar els existents: quan
// l'usuari deia «treu-me el recordatori repetit» el vent no podia.
// Tres operacions: cancel (esborra), reschedule (nova hora), pause/resume.
var remindEditToolSpec = llm.ToolSpec{
	Name: "remind_edit",
	Description: "Edita o elimina un recordatori o automatització ja existent de l'usuari. Fes-la servir quan l'usuari vulgui treure, ajornar, pausar o reprendre un pendent: primer llista els ids amb remind_list i després aplica l'operació sobre l'id exacte. Operacions: «cancel» (l'esborra definitivament), «reschedule» (li posa una hora nova, mateix text), «pause» (el desactiva sense esborrar-lo) i «resume» (el reactiva). Sempre confirma a l'usuari què has tocat i com queda.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"id":      map[string]any{"type": "string", "description": "id del recordatori (el dóna remind_list)"},
		"op":      map[string]any{"type": "string", "enum": []string{"cancel", "reschedule", "pause", "resume"}, "description": "operació"},
		"at":      map[string]any{"type": "string", "description": "només amb reschedule: la nova hora (RFC3339, «2006-01-02 15:04», «+2h»…)"},
	}, "required": []string{"id", "op"}},
}

// remindEdit aplica l'operació sobre un job visible per aquesta sessió.
// La propietat és la mateixa que el panell: windRoutineVisibleID.
func (s *Server) remindEdit(id, op, at string) (string, error) {
	if s.jobs == nil {
		return "", fmt.Errorf("no hi ha jobs")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id buit")
	}
	if !s.windRoutineVisibleID(id) {
		return "", fmt.Errorf("no hi ha cap recordatori amb id %s", id)
	}
	all, err := s.jobs.List()
	if err != nil {
		return "", err
	}
	var job *jobs.Job
	for i := range all {
		if all[i].ID == id {
			job = &all[i]
			break
		}
	}
	if job == nil {
		return "", fmt.Errorf("no hi ha cap recordatori amb id %s", id)
	}
	switch strings.TrimSpace(op) {
	case "cancel":
		if err := s.jobs.Delete(id); err != nil {
			return "", err
		}
		return fmt.Sprintf("Esborrat: «%s» ja no sonarà.", strings.TrimSpace(firstNonEmpty(job.Prompt, job.Name))), nil
	case "reschedule":
		moment, err := parseRemindAt(strings.TrimSpace(at), time.Now())
		if err != nil {
			return "", fmt.Errorf("hora no vàlida: %v", err)
		}
		if moment.Before(time.Now()) {
			return "", fmt.Errorf("l'hora nova (%s) ja és passada", moment.Format("02/01 15:04"))
		}
		if moment.After(time.Now().Add(remindMaxAhead)) {
			return "", fmt.Errorf("més enllà de %d dies", int(remindMaxAhead.Hours()/24))
		}
		job.NextAt = &moment
		job.Enabled = true
		if _, err := s.jobs.Save(*job); err != nil {
			return "", err
		}
		return fmt.Sprintf("Reprogramat: «%s» sonarà %s.", strings.TrimSpace(firstNonEmpty(job.Prompt, job.Name)), moment.Format("02/01 15:04")), nil
	case "pause":
		job.Enabled = false
		if _, err := s.jobs.Save(*job); err != nil {
			return "", err
		}
		return fmt.Sprintf("Pausat: «%s» no sonarà fins que el reprènguis.", strings.TrimSpace(firstNonEmpty(job.Prompt, job.Name))), nil
	case "resume":
		job.Enabled = true
		if _, err := s.jobs.Save(*job); err != nil {
			return "", err
		}
		return fmt.Sprintf("Reactivat: «%s» torna a estar actiu.", strings.TrimSpace(firstNonEmpty(job.Prompt, job.Name))), nil
	default:
		return "", fmt.Errorf("operació desconeguda: %s (usa cancel, reschedule, pause o resume)", op)
	}
}

// remindOwner és el compte d'aquesta sessió ("" en local o per a tothom).
func (s *Server) remindOwner() string {
	if s.user != nil && !s.user.Tothom {
		return s.user.Name
	}
	return ""
}

type remindServerKey struct{}

// withRemindServer lliga el context del torn a la sessió real del Server
// (amb els vents de l'usuari); sense això, registerRemindTool usava el
// Server d'arrencada i per a un usuari multiusuari s.winds estava buit
// ("vent inexistent").
func (s *Server) withRemindServer(ctx context.Context) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, remindServerKey{}, s)
}

// registerRemindTool exposa l'eina remind al loop (la criden els vents).
func (s *Server) registerRemindTool() {
	agent.RegisterExtraCtx(remindListToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a struct {
			Wind string `json:"wind"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		items, err := target.remindListItems(strings.TrimSpace(a.Wind))
		if err != nil {
			return "", err
		}
		if len(items) == 0 {
			return "Cap recordatori ni automatització pendent.", nil
		}
		var sb strings.Builder
		for i, it := range items {
			if i > 0 {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "- %s · %s · %s", it["moment"], it["vent"], it["titol"])
			if it["detall"] != "" {
				fmt.Fprintf(&sb, " · %s", it["detall"])
			}
			if it["estat"] != "actiu" {
				fmt.Fprintf(&sb, " · [%s]", it["estat"])
			}
		}
		return sb.String(), nil
	})
	agent.RegisterExtraCtx(remindEditToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a struct {
			ID string `json:"id"`
			Op string `json:"op"`
			At string `json:"at"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		return target.remindEdit(a.ID, a.Op, a.At)
	})
	agent.RegisterExtraCtx(remindToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a struct {
			Wind      string          `json:"wind"`
			At        string          `json:"at"`
			Text      string          `json:"text"`
			Task      string          `json:"task"`
			RetryRaw  json.RawMessage `json:"retry_minutes"`
			FollowRaw json.RawMessage `json:"followup"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", fmt.Errorf("arguments il·legibles")
		}
		// Els models sovint envien "true" o "15" com a text: s'accepta igual.
		Retry, Followup := looseInt(a.RetryRaw), looseBool(a.FollowRaw)
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		j, err := saveWindReminderFull(target, a.Wind, a.At, a.Text, a.Task, Retry, Followup)
		if err != nil {
			log.Printf("remind wind=%s at=%q followup=%v: %v", a.Wind, a.At, Followup, err)
			return "", err
		}
		if Followup {
			return "Fet: seguiment programat per al " + j.NextAt.Format("02/01 15:04") + ". Digues-li en mitja frase que hi tornaràs (l'app li mostra el botó per desfer-ho).", nil
		}
		return "Fet: avisaré el " + j.NextAt.Format("02/01 15:04") + ".", nil
	})
}

// postWindMessage fa parlar el vent: el text entra al seu xat com si
// l'hagués dit ell, es desa i n'aprèn. Ho comparteixen el cor i els
// recordatoris.
func postWindMessage(s *Server, w companion.Wind, source, text string) {
	postWindMessageJob(s, w, source, "", text)
}

// postWindMessageJob és postWindMessage amb l'id del recordatori que parla.
func postWindMessageJob(s *Server, w companion.Wind, source, job, text string) {
	hist := windHistory(s, w.ID)
	hist.mu.Lock()
	hist.messages = podaHistorialVent(append(hist.messages, llm.Message{Role: "assistant", Content: text, Ts: time.Now().Unix()}))
	history := append([]llm.Message(nil), hist.messages...)
	hist.mu.Unlock()
	_ = saveWindHistory(s, w, history)
	goBackground(func() { s.windLearn(w, source, text) })
	// Registre per a l'app (notificació local): el missatge és proactiu,
	// l'usuari pot tenir l'app en segon pla. Best-effort, mai no bloqueja.
	recordNovetatJob(s, w, source, job, text)
}

// executeWindRemind dispara un recordatori: el text entra al xat del vent,
// la sortida (notificació) el porta i el job s'apaga (una sola execució).
func (s *Server) executeWindRemind(ctx context.Context, job jobs.Job) (jobs.Run, error) {
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
	if strings.TrimSpace(job.Task) != "" {
		return s.executeWindAction(ctx, job, w)
	}
	postWindMessageJob(s, w, "remind", job.ID, text)
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
		" l'usuari encara que l'app estigui tancada." +
		" Si l'usuari vol que a aquella hora FACIS alguna cosa (cercar un resultat, un preu, una notícia)," +
		" posa-ho a task: a l'hora faràs un torn complet amb cerca web i diràs el resultat" +
		" (amb retry_minutes si pot no estar llest). Mai no el facis creure que només és un avís de text." +
		" Només avisos reals; no programis" +
		" mai res sense motiu ni avisis del que ja has dit en aquest torn."
}

// followupRoom: un vent no s'omple de seguiments. Com a molt
// followupMaxPending de pendents i cap altre a menys de tres hores del nou
// (dos seguiments seguits del mateix tema són soroll, no ajuda).
func (s *Server) followupRoom(windID string, at time.Time) error {
	list, err := s.jobs.List()
	if err != nil {
		return err
	}
	owner := s.remindOwner()
	pending := 0
	for _, j := range list {
		if !j.Followup || !j.Enabled || j.WindOwner != owner || j.NextAt == nil {
			continue
		}
		if id, ok := remindJobWind(j.ID); !ok || id != windID {
			continue
		}
		pending++
		if d := j.NextAt.Sub(at); d < 3*time.Hour && d > -3*time.Hour {
			return fmt.Errorf("ja tens un seguiment programat el %s: no en posis un altre tan a prop", j.NextAt.Format("02/01 15:04"))
		}
	}
	if pending >= followupMaxPending {
		return fmt.Errorf("ja tens %d seguiments pendents: no n'ofereixis més fins que passin", pending)
	}
	return nil
}

// windFollowupBrief és el punt de proactivitat: el vent fa de bon assistent i
// s'ofereix a tornar-hi quan l'usuari esmenta una cosa seva amb data. Buit si
// l'usuari ho ha apagat per a aquest vent.
func windFollowupBrief(w companion.Wind) string {
	if !w.FollowupsOn() {
		return ""
	}
	return "\n\nSEGUIMENT PROACTIU (fes de bon assistent): si l'usuari esmenta una cosa SEVA amb data o moment que encara ha de passar" +
		" (un examen, el metge, una entrevista, un viatge, una entrega, una reunió important, una cita), pots oferir-te a fer-ne el seguiment" +
		" sense que t'ho demani: crida remind amb followup=true i at una mica després que hagi passat (o la vigília, si l'hi pots ajudar a preparar)." +
		" text és la pregunta o l'avís curt que li faràs («Com ha anat l'entrevista?»); si aquell moment pots ajudar de debò, posa a task què faràs" +
		" (p. ex. «Recorda-li els tres punts que vam preparar i pregunta-li com li ha anat»)." +
		" Com a molt UN seguiment per torn, i mai per a coses trivials, per a coses que ja ha resolt, si ja n'hi ha un del mateix tema o si l'usuari no vol." +
		" Després, en mitja frase, digues-li què faràs i quan (l'app li ensenya un botó per desfer-ho); no li preguntis permís abans."
}

// looseBool llegeix true, "true", "sí", 1… (els models no sempre respecten el tipus).
func looseBool(raw json.RawMessage) bool {
	v := strings.Trim(strings.ToLower(strings.TrimSpace(string(raw))), `"`)
	switch v {
	case "true", "1", "yes", "si", "sí":
		return true
	}
	return false
}

// looseInt llegeix 15 o "15"; qualsevol altra cosa és 0.
func looseInt(raw json.RawMessage) int {
	v := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	n, err := strconv.Atoi(strings.TrimSuffix(v, ".0"))
	if err != nil {
		return 0
	}
	return n
}
