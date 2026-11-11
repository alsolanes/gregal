package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/jobs"
	"gregal/internal/llm"
)

// «Fes-ho així la propera vegada» (B8, primera versió): l'usuari ensenya una
// rutina amb instruccions en text i, si vol, li marca un resultat com a model.
// Tot queda desat a la mateixa rutina (Job.Howto i Job.Refs), es pot veure,
// editar, pausar i esborrar com qualsevol altra, i entra a la consigna només
// d'aquella rutina quan corre. No hi ha aprenentatge per vídeo ni
// demostracions: només text que l'usuari pot llegir i corregir.
//
//	POST /api/winds/rutines/edit  {id, name?, prompt?, howto?, kind?, time?, interval_h?, keep_refs?}
//	POST /api/winds/rutines/teach {id, instruction?, ref_run?, ref_message?}
//	eina routine_teach            {wind, routine?, instruction, use_last_result?}

const (
	rutinaMaxHowto   = 2000
	rutinaMaxRefs    = 3
	rutinaMaxExcerpt = 800
)

// isWindRutina: una rutina d'un vent (no el cor ni un recordatori).
func isWindRutina(j jobs.Job) bool {
	return j.Wind != "" && (j.Kind == "daily" || j.Kind == "interval") &&
		!strings.HasPrefix(j.ID, remindJobPrefix) && !strings.HasPrefix(j.ID, respJobPrefix)
}

// rutinaHowtoBrief és el tros de consigna amb el que l'usuari ha ensenyat.
// Buit si no hi ha res: les rutines antigues corren exactament igual.
func rutinaHowtoBrief(j jobs.Job) string {
	howto := strings.TrimSpace(j.Howto)
	if howto == "" && len(j.Refs) == 0 {
		return ""
	}
	var b strings.Builder
	if howto != "" {
		b.WriteString("\n\nHOW THE USER WANTS THIS ROUTINE DONE (they taught you this; follow it unless it is impossible):\n" + howto)
	}
	header := false
	for _, r := range j.Refs {
		if strings.TrimSpace(r.Excerpt) == "" {
			continue
		}
		if !header {
			header = true
			b.WriteString("\n\nRESULTS THE USER MARKED AS A GOOD EXAMPLE (match their shape and level of detail, not their facts — facts must be fresh):")
		}
		b.WriteString("\n---\n" + r.Excerpt)
	}
	return b.String()
}

// rutinaAddHowto afegeix una instrucció (una línia «- …») sense duplicar-la.
func rutinaAddHowto(howto, instruction string) (string, error) {
	instruction = strings.Join(strings.Fields(instruction), " ")
	instruction = strings.TrimPrefix(instruction, "- ")
	if instruction == "" {
		return howto, nil
	}
	line := "- " + instruction
	for _, l := range strings.Split(howto, "\n") {
		if strings.EqualFold(strings.TrimSpace(l), line) {
			return howto, nil
		}
	}
	out := strings.TrimSpace(howto)
	if out != "" {
		out += "\n"
	}
	out += line
	if len([]rune(out)) > rutinaMaxHowto {
		return howto, fmt.Errorf("les instruccions d'aquesta rutina ja són massa llargues (màxim %d): edita-les i resumeix-les", rutinaMaxHowto)
	}
	return out, nil
}

// rutinaAddRef desa un resultat com a exemple (el més nou primer, com a
// molt rutinaMaxRefs). El mateix id no es repeteix.
func rutinaAddRef(refs []jobs.ResultRef, r jobs.ResultRef) []jobs.ResultRef {
	out := []jobs.ResultRef{r}
	for _, x := range refs {
		if x.ID != r.ID {
			out = append(out, x)
		}
	}
	if len(out) > rutinaMaxRefs {
		out = out[:rutinaMaxRefs]
	}
	return out
}

// rutinaRefFromRun busca un run de la rutina i en desa l'extracte.
func (s *Server) rutinaRefFromRun(j jobs.Job, runID string) (jobs.ResultRef, error) {
	runs, err := s.jobs.Runs(j.ID, 50)
	if err != nil {
		return jobs.ResultRef{}, err
	}
	for _, r := range runs {
		if (runID == "" || r.ID == runID) && r.Status == "ok" && strings.TrimSpace(r.Output) != "" {
			return jobs.ResultRef{Kind: "run", ID: r.ID, Excerpt: clipRunes(strings.TrimSpace(r.Output), rutinaMaxExcerpt), At: r.FinishedAt}, nil
		}
	}
	if runID == "" {
		return jobs.ResultRef{}, fmt.Errorf("aquesta rutina encara no té cap resultat")
	}
	return jobs.ResultRef{}, fmt.Errorf("no trobo aquest resultat de la rutina")
}

// rutinaRefFromMessage busca un missatge del vent al seu fil (per id, o
// l'últim del vent si l'id és buit).
func (s *Server) rutinaRefFromMessage(windID, msgID string) (jobs.ResultRef, error) {
	hist := windHistory(s, windID)
	hist.mu.Lock()
	msgs := append([]llm.Message(nil), hist.messages...)
	hist.mu.Unlock()
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "assistant" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		id := messageID(m)
		if msgID == "" || id == msgID {
			at := time.Unix(m.Ts, 0)
			return jobs.ResultRef{Kind: "message", ID: id, Excerpt: clipRunes(strings.TrimSpace(m.Content), rutinaMaxExcerpt), At: at}, nil
		}
	}
	return jobs.ResultRef{}, fmt.Errorf("no trobo aquest missatge al fil del vent")
}

// rutinaTeach desa una instrucció i/o un resultat a la rutina (mateix id).
func (s *Server) rutinaTeach(j jobs.Job, instruction string, ref *jobs.ResultRef) (jobs.Job, error) {
	howto, err := rutinaAddHowto(j.Howto, instruction)
	if err != nil {
		return jobs.Job{}, err
	}
	if howto == j.Howto && ref == nil && strings.TrimSpace(instruction) == "" {
		return jobs.Job{}, fmt.Errorf("cal una instrucció o un resultat")
	}
	j.Howto = howto
	if ref != nil {
		j.Refs = rutinaAddRef(j.Refs, *ref)
	}
	return s.jobs.Save(j)
}

// ownWindRutina: rutina d'un vent d'aquest compte (no cors ni recordatoris).
func (s *Server) ownWindRutina(id string) (jobs.Job, bool) {
	j, ok := s.ownWindJob(id)
	if !ok || !isWindRutina(j) {
		return jobs.Job{}, false
	}
	return j, true
}

func (s *Server) handleWindsRutinaTeach(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID          string `json:"id"`
		Instruction string `json:"instruction"`
		RefRun      string `json:"ref_run"`
		RefMessage  string `json:"ref_message"`
		RefLast     bool   `json:"ref_last"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	j, ok := s.ownWindRutina(req.ID)
	if !ok {
		http.Error(w, "unknown routine", http.StatusNotFound)
		return
	}
	var ref *jobs.ResultRef
	switch {
	case strings.TrimSpace(req.RefMessage) != "":
		got, err := s.rutinaRefFromMessage(j.Wind, strings.TrimSpace(req.RefMessage))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ref = &got
	case strings.TrimSpace(req.RefRun) != "" || req.RefLast:
		got, err := s.rutinaRefFromRun(j, strings.TrimSpace(req.RefRun))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ref = &got
	}
	saved, err := s.rutinaTeach(j, req.Instruction, ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"job": rutinaFromJob(s, saved)})
}

// handleWindsRutinaEdit edita una rutina pròpia sense canviar-ne l'id.
// Camps absents = sense canvis. keep_refs: ids dels exemples que es queden
// (absent = tots; [] = cap).
func (s *Server) handleWindsRutinaEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID        string    `json:"id"`
		Name      *string   `json:"name"`
		Prompt    *string   `json:"prompt"`
		Howto     *string   `json:"howto"`
		Kind      *string   `json:"kind"`
		Time      *string   `json:"time"`
		IntervalH *float64  `json:"interval_h"`
		KeepRefs  *[]string `json:"keep_refs"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	j, ok := s.ownWindRutina(req.ID)
	if !ok {
		http.Error(w, "unknown routine", http.StatusNotFound)
		return
	}
	if req.Name != nil {
		if n := strings.TrimSpace(*req.Name); n != "" {
			j.Name = n
		}
	}
	if req.Prompt != nil {
		p := strings.TrimSpace(*req.Prompt)
		if p == "" || len([]rune(p)) > rutinaMaxPrompt {
			http.Error(w, "cal dir què ha de fer (màxim 2000)", http.StatusBadRequest)
			return
		}
		j.Prompt = p
	}
	if req.Howto != nil {
		h := strings.TrimSpace(*req.Howto)
		if len([]rune(h)) > rutinaMaxHowto {
			http.Error(w, "instruccions massa llargues (màxim 2000)", http.StatusBadRequest)
			return
		}
		j.Howto = h
	}
	if req.Kind != nil {
		j.Kind = strings.TrimSpace(*req.Kind)
	}
	if req.Time != nil {
		j.Time = strings.TrimSpace(*req.Time)
	}
	if req.IntervalH != nil {
		j.IntervalH = *req.IntervalH
	}
	switch j.Kind {
	case "daily":
		if _, err := time.Parse("15:04", j.Time); err != nil {
			http.Error(w, "hora amb format HH:MM", http.StatusBadRequest)
			return
		}
	case "interval":
		if j.IntervalH <= 0 || j.IntervalH > 24*30 {
			http.Error(w, "interval entre 0 i 720 hores", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "kind ha de ser daily o interval", http.StatusBadRequest)
		return
	}
	if req.KeepRefs != nil {
		keep := map[string]bool{}
		for _, id := range *req.KeepRefs {
			keep[id] = true
		}
		out := []jobs.ResultRef{}
		for _, ref := range j.Refs {
			if keep[ref.ID] {
				out = append(out, ref)
			}
		}
		j.Refs = out
	}
	saved, err := s.jobs.Save(j)
	if err != nil {
		http.Error(w, "cannot save", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"job": rutinaFromJob(s, saved)})
}

var rutinaTeachToolSpec = llm.ToolSpec{
	Name: "routine_teach",
	Description: "Desa a una rutina teva (una feina programada que ja existeix) com vol l'usuari que la facis a partir d'ara. Fes-la servir quan l'usuari digui «fes-ho així la propera vegada», «a partir d'ara a l'informe posa…», o corregeixi la forma d'un resultat d'una rutina. instruction és la indicació curta i concreta, en la llengua de l'usuari. use_last_result=true marca el teu últim missatge com a exemple de resultat que li ha agradat. Si tens més d'una rutina, posa routine (id o nom; si no el saps, crida-la sense i et dirà quines hi ha). Només guarda text: no aprens de vídeos ni de demostracions, no ho prometis. Després digues a l'usuari en mitja frase què has desat; ho pot revisar i editar a Programacions.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind":            map[string]any{"type": "string"},
		"routine":         map[string]any{"type": "string"},
		"instruction":     map[string]any{"type": "string"},
		"use_last_result": map[string]any{"type": "boolean"},
	}, "required": []string{"wind", "instruction"}},
}

// rutinaTeachTool resol la rutina i hi desa la instrucció. Torna el text per
// al model.
func (s *Server) rutinaTeachTool(windRef, routine, instruction string, useLast bool) (string, error) {
	if s.winds == nil || s.jobs == nil {
		return "", fmt.Errorf("rutines no disponibles")
	}
	w, ok := windByRef(s, strings.TrimSpace(windRef))
	if !ok {
		return "", fmt.Errorf("vent inexistent")
	}
	if strings.TrimSpace(instruction) == "" {
		return "", fmt.Errorf("instruction buida")
	}
	all, err := s.jobs.List()
	if err != nil {
		return "", err
	}
	var mine []jobs.Job
	for _, j := range all {
		if j.Wind == w.ID && isWindRutina(j) && s.canSeeWindRoutine(j) {
			mine = append(mine, j)
		}
	}
	if len(mine) == 0 {
		return "", fmt.Errorf("no tens cap rutina programada: digues a l'usuari que primer en creï una (Programacions → Nova rutina) o desa-ho com a preferència a la teva memòria")
	}
	var target *jobs.Job
	routine = strings.TrimSpace(routine)
	for i := range mine {
		if routine != "" && (mine[i].ID == routine || strings.EqualFold(strings.TrimSpace(mine[i].Name), routine)) {
			target = &mine[i]
			break
		}
	}
	if target == nil {
		if routine == "" && len(mine) == 1 {
			target = &mine[0]
		} else {
			var names []string
			for _, j := range mine {
				names = append(names, j.ID+" («"+j.Name+"»)")
			}
			return "", fmt.Errorf("tria la rutina amb routine: %s", strings.Join(names, ", "))
		}
	}
	var ref *jobs.ResultRef
	if useLast {
		if got, err := s.rutinaRefFromMessage(w.ID, ""); err == nil {
			ref = &got
		}
	}
	saved, err := s.rutinaTeach(*target, instruction, ref)
	if err != nil {
		return "", err
	}
	msg := "Desat a la rutina «" + saved.Name + "»: la propera vegada ho faràs així."
	if ref != nil {
		msg += " També hi he guardat el teu últim missatge com a exemple."
	}
	return msg, nil
}

// registerRutinaTeachTool exposa routine_teach als vents (com remind).
func (s *Server) registerRutinaTeachTool() {
	agent.RegisterExtraCtx(rutinaTeachToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a struct {
			Wind        string          `json:"wind"`
			Routine     string          `json:"routine"`
			Instruction string          `json:"instruction"`
			UseLast     json.RawMessage `json:"use_last_result"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", fmt.Errorf("arguments il·legibles")
		}
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		return target.rutinaTeachTool(a.Wind, a.Routine, a.Instruction, looseBool(a.UseLast))
	})
}
