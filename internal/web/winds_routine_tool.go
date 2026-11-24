package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/jobs"
	"gregal/internal/llm"
)

// Rutines des del xat: «cada matí a les 8 envia'm les notícies de…». Abans
// només es podia des del formulari de l'app; el vent, sense eina, posava una
// setmana de recordatoris puntuals i deia que no podia. L'eina routine crea,
// llista i atura rutines del mateix vent (els mateixos jobs «rutina-…» que
// el formulari: es veuen i s'editen a Automatitzacions).
//
//	eina routine {wind, action: create|list|stop, task?, time?, repeat?, days?, every_hours?, name?, routine?}

var routineToolSpec = llm.ToolSpec{
	Name: "routine",
	Description: "Rutines teves que es repeteixen soles al servidor, encara que l'app estigui tancada: «cada matí a les 8 les notícies de…», «cada dilluns el resum de…», «cada 3 hores mira el preu de…». " +
		"action=create amb task (què has de fer cada cop, concret, p. ex. «Busca les 5 notícies més importants de Catalunya d'avui i resumeix-les en una línia cadascuna amb l'enllaç»), time HH:MM i repeat: daily (cada dia), weekdays (de dilluns a divendres), weekends, o days amb days=[\"mon\",\"wed\"]; o every_hours per a cada N hores. " +
		"A l'hora faràs un torn complet amb cerca web i el resultat arribarà al teu xat. action=list per veure les teves rutines; action=stop amb routine (id o nom) per aturar-ne una. " +
		"action=follow amb name (un equip o qualsevol cosa que la persona vol seguir, p. ex. «Barça») i kind team|other: cada dia en buscaràs el proper partit o la novetat i sortirà a Avui; action=unfollow amb name per deixar-ho de seguir. " +
		"Fes servir SEMPRE el teu id de vent. Per a una sola vegada fes servir remind, no routine. CRIDA L'EINA ABANS de dir que ho has fet.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind":        map[string]any{"type": "string"},
		"action":      map[string]any{"type": "string", "enum": []string{"create", "list", "stop", "follow", "unfollow"}},
		"kind":        map[string]any{"type": "string", "enum": []string{"team", "other"}},
		"task":        map[string]any{"type": "string"},
		"time":        map[string]any{"type": "string"},
		"repeat":      map[string]any{"type": "string", "enum": []string{"daily", "weekdays", "weekends", "days"}},
		"days":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"every_hours": map[string]any{"type": "number"},
		"name":        map[string]any{"type": "string"},
		"routine":     map[string]any{"type": "string"},
	}, "required": []string{"wind", "action"}},
}

var routineDayNames = map[string]int{
	"sun": 0, "sunday": 0, "dg": 0, "diumenge": 0,
	"mon": 1, "monday": 1, "dl": 1, "dilluns": 1,
	"tue": 2, "tuesday": 2, "dt": 2, "dimarts": 2,
	"wed": 3, "wednesday": 3, "dc": 3, "dimecres": 3,
	"thu": 4, "thursday": 4, "dj": 4, "dijous": 4,
	"fri": 5, "friday": 5, "dv": 5, "divendres": 5,
	"sat": 6, "saturday": 6, "ds": 6, "dissabte": 6,
}

// routineDays tradueix repeat/days a la llista de Job.Days (buit = cada dia).
func routineDays(repeat string, days []string) ([]int, error) {
	switch strings.ToLower(strings.TrimSpace(repeat)) {
	case "", "daily":
		if len(days) == 0 {
			return nil, nil
		}
	case "weekdays":
		return []int{1, 2, 3, 4, 5}, nil
	case "weekends":
		return []int{0, 6}, nil
	case "days":
	default:
		return nil, fmt.Errorf("repeat ha de ser daily, weekdays, weekends o days")
	}
	seen := map[int]bool{}
	var out []int
	for _, d := range days {
		n, ok := routineDayNames[strings.ToLower(strings.Trim(strings.TrimSpace(d), "."))]
		if !ok {
			return nil, fmt.Errorf("dia desconegut: %q (fes servir mon, tue, wed, thu, fri, sat, sun)", d)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("amb repeat=days cal days, p. ex. [\"mon\",\"thu\"]")
	}
	return out, nil
}

// routineWhen és la frase curta de quan corre («feiners a les 08:00»).
func routineWhen(s *Server, j jobs.Job) string {
	if j.Kind == "interval" {
		return fmt.Sprintf(s.tr("cada %g h", "every %g h"), j.IntervalH)
	}
	days := ""
	switch {
	case len(j.Days) == 0:
		days = s.tr("cada dia", "every day")
	case len(j.Days) == 5 && !dayIn(j.Days, 0) && !dayIn(j.Days, 6):
		days = s.tr("feiners", "weekdays")
	case len(j.Days) == 2 && dayIn(j.Days, 0) && dayIn(j.Days, 6):
		days = s.tr("caps de setmana", "weekends")
	default:
		ca := []string{"dg", "dl", "dt", "dc", "dj", "dv", "ds"}
		en := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		var names []string
		for _, d := range j.Days {
			if d >= 0 && d < 7 {
				names = append(names, s.tr(ca[d], en[d]))
			}
		}
		days = strings.Join(names, ", ")
	}
	return days + s.tr(" a les ", " at ") + j.Time
}

func dayIn(days []int, d int) bool {
	for _, x := range days {
		if x == d {
			return true
		}
	}
	return false
}

type routineArgs struct {
	Wind       string   `json:"wind"`
	Action     string   `json:"action"`
	Task       string   `json:"task"`
	Time       string   `json:"time"`
	Repeat     string   `json:"repeat"`
	Days       []string `json:"days"`
	EveryHours float64  `json:"every_hours"`
	Name       string   `json:"name"`
	Routine    string   `json:"routine"`
	Kind       string   `json:"kind"`
}

// routineTool fa l'acció i torna el text per al model.
func (s *Server) routineTool(a routineArgs, now time.Time) (string, error) {
	if s.winds == nil || s.jobs == nil {
		return "", fmt.Errorf("rutines no disponibles")
	}
	w, ok := windByRef(s, strings.TrimSpace(a.Wind))
	if !ok {
		return "", fmt.Errorf("vent inexistent: fes servir el teu id de vent")
	}
	mine := func() ([]jobs.Job, error) {
		all, err := s.jobs.List()
		if err != nil {
			return nil, err
		}
		var out []jobs.Job
		for _, j := range all {
			if j.Wind == w.ID && isWindRutina(j) && s.canSeeWindRoutine(j) {
				out = append(out, j)
			}
		}
		return out, nil
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "list":
		list, err := mine()
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "No tens cap rutina.", nil
		}
		var b strings.Builder
		for _, j := range list {
			state := ""
			if !j.Enabled {
				state = " (aturada)"
			}
			fmt.Fprintf(&b, "- %s «%s»: %s%s — %s\n", j.ID, j.Name, routineWhen(s, j), state, oneLine(j.Prompt, 160))
		}
		return strings.TrimSpace(b.String()), nil
	case "stop":
		list, err := mine()
		if err != nil {
			return "", err
		}
		ref := strings.TrimSpace(a.Routine)
		var target *jobs.Job
		for i := range list {
			if ref != "" && (list[i].ID == ref || strings.EqualFold(strings.TrimSpace(list[i].Name), ref)) {
				target = &list[i]
				break
			}
		}
		if target == nil && ref == "" && len(list) == 1 {
			target = &list[0]
		}
		if target == nil {
			if len(list) == 0 {
				return "", fmt.Errorf("no tens cap rutina per aturar")
			}
			var names []string
			for _, j := range list {
				names = append(names, j.ID+" («"+j.Name+"»)")
			}
			return "", fmt.Errorf("tria quina amb routine: %s", strings.Join(names, ", "))
		}
		if err := s.jobs.Delete(target.ID); err != nil {
			return "", err
		}
		return "Aturada i esborrada la rutina «" + target.Name + "».", nil
	case "follow", "unfollow":
		return s.followTool(w, a)
	case "create":
	default:
		return "", fmt.Errorf("action ha de ser create, list o stop")
	}

	task := strings.TrimSpace(a.Task)
	if task == "" {
		return "", fmt.Errorf("cal task: què has de fer cada cop")
	}
	if len([]rune(task)) > rutinaMaxPrompt {
		return "", fmt.Errorf("task massa llarga (màxim %d caràcters)", rutinaMaxPrompt)
	}
	job := jobs.Job{Prompt: task, Enabled: true, Notify: false, Wind: w.ID}
	if s.user != nil && !s.user.Tothom {
		job.WindOwner = s.user.Name
	}
	if a.EveryHours > 0 {
		if a.EveryHours < 1 || a.EveryHours > 24*30 {
			return "", fmt.Errorf("every_hours entre 1 i 720")
		}
		job.Kind, job.IntervalH = "interval", a.EveryHours
	} else {
		hm, err := time.Parse("15:04", strings.TrimSpace(a.Time))
		if err != nil {
			return "", fmt.Errorf("cal time amb format HH:MM (o every_hours)")
		}
		days, err := routineDays(a.Repeat, a.Days)
		if err != nil {
			return "", err
		}
		job.Kind, job.Time, job.Days = "daily", hm.Format("15:04"), days
	}
	// Que no corri en crear-la: la primera, a la propera hora que toqui.
	created := now
	job.LastAt = &created
	name := strings.TrimSpace(a.Name)
	if name == "" {
		name = oneLine(task, 60)
	}
	job.Name = name
	job.ID = fmt.Sprintf("rutina-%s-%d", w.ID, now.UnixNano()/1e6)
	saved, err := s.jobs.Save(job)
	if err != nil {
		return "", err
	}
	next := jobs.NextDue(saved, now)
	msg := fmt.Sprintf("Rutina creada: «%s», %s. ", saved.Name, routineWhen(s, saved))
	if !next.IsZero() {
		msg += "La propera: " + next.Format("Mon 2 Jan 15:04") + ". "
	}
	msg += "Es veu i s'edita a Automatitzacions; per aturar-la, routine action=stop."
	return msg, nil
}

// registerRoutineTool exposa routine als vents (com remind i routine_teach).
func (s *Server) registerRoutineTool() {
	agent.RegisterExtraCtx(routineToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a routineArgs
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", fmt.Errorf("arguments il·legibles")
		}
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		return target.routineTool(a, time.Now())
	})
}
