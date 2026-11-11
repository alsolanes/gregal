package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Registre d'especialitats i invocació de vents.
//
// Un agent de més amunt (el xat principal, o qui coordini) ha de poder
// trobar el vent que sap fer una feina i encarregar-l'hi, sense que la
// persona hagi d'obrir el xat del vent. L'especialitat d'un vent és el que
// diu el seu rol més el que ha après (els noms, títols i descripcions dels
// coneixements i del manual d'ofici). La tria és local i explicable (cap
// crida al model): paraules de la tasca que surten al rol o als
// coneixements, i el kit de l'ofici.
//
//	GET /api/winds/specialists?task=… → {specialists:[{id,name,role,score,why,coneixements}]}
//	eina `wind` {action:list|ask, wind?, task}

type specialistCard struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Role         string           `json:"role"`
	Kit          string           `json:"kit,omitempty"`
	Score        float64          `json:"score"`
	Why          []string         `json:"why,omitempty"`
	Coneixements []map[string]any `json:"coneixements"`
	Lessons      int              `json:"lessons"`
}

// paraulesBuides no diuen res de l'ofici.
var paraulesBuides = map[string]bool{
	"fes": true, "fer": true, "make": true, "build": true, "crea": true, "create": true, "una": true, "uns": true, "les": true,
	"els": true, "per": true, "que": true, "amb": true, "the": true, "and": true, "for": true, "with": true, "this": true,
	"aquest": true, "aquesta": true, "molt": true, "petit": true, "petita": true, "small": true, "nou": true, "nova": true, "new": true,
	"vull": true, "want": true, "need": true, "please": true, "sisplau": true, "can": true, "pots": true, "puc": true,
	"del": true, "dels": true, "des": true, "com": true, "how": true, "what": true, "sobre": true, "about": true,
	"mobil": true, "phone": true,
}

// paraulesDe treu les paraules amb sentit (minúscules, sense accents).
func paraulesDe(text string) []string {
	t := norm.NFD.String(strings.ToLower(text))
	var b strings.Builder
	for _, r := range t {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(b.String()) {
		if len(w) < 3 || paraulesBuides[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// arrel compara paraules per la seva arrel curta (joc/jocs, game/games,
// web/webs): els 4 primers caràcters, o la paraula sencera si és curta.
func arrel(w string) string {
	r := []rune(w)
	if len(r) > 4 {
		return string(r[:4])
	}
	return strings.TrimSuffix(w, "s")
}

// coincidencies diu quines paraules de la tasca surten al text.
func coincidencies(task []string, text string) []string {
	have := map[string]bool{}
	for _, w := range paraulesDe(text) {
		have[arrel(w)] = true
	}
	var out []string
	for _, w := range task {
		if have[arrel(w)] {
			out = append(out, w)
		}
	}
	return out
}

// windSpecialists ordena els vents per com d'adequats són per a la tasca
// (amb task buida, tots amb puntuació 0, en l'ordre de la llista).
func (s *Server) windSpecialists(task string) []specialistCard {
	winds, _ := s.winds.List()
	return s.rankWindSpecialists(winds, task)
}

func (s *Server) rankWindSpecialists(winds []companion.Wind, task string) []specialistCard {
	winds = withoutGregal(winds)
	words := paraulesDe(task)
	taskKit := inferKit(task)
	out := make([]specialistCard, 0, len(winds))
	for _, w := range winds {
		c := specialistCard{ID: w.ID, Name: w.Name, Role: w.Role, Kit: kitOf(w), Coneixements: []map[string]any{}}
		for _, m := range coincidencies(words, w.Role) {
			c.Score += 2
			c.Why = append(c.Why, "role:"+m)
		}
		for _, sk := range s.windSkills(w.ID) {
			c.Coneixements = append(c.Coneixements, map[string]any{"nom": sk.Nom, "titol": sk.Titol, "descripcio": sk.Descripcio})
			if sk.Nom == windLessonsSkill {
				c.Lessons = strings.Count(sk.Cos, "\n- ")
				continue
			}
			for _, m := range coincidencies(words, sk.Nom+" "+sk.Titol+" "+sk.Descripcio) {
				c.Score += 1.5
				c.Why = append(c.Why, sk.Nom+":"+m)
			}
		}
		if taskKit != "" && taskKit == c.Kit {
			c.Score += 2
			c.Why = append(c.Why, "kit:"+taskKit)
		}
		// Un vent que ja ha fet feina d'aquest ofici en sap més.
		if c.Score > 0 && c.Lessons > 0 {
			c.Score += 0.5
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// handleWindSpecialists exposa el registre (per a l'app i els coordinadors).
func (s *Server) handleWindSpecialists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{"specialists": s.windSpecialists(r.URL.Query().Get("task"))})
}

// runWindTool és l'eina `wind` del xat principal.
func (s *Server) runWindTool(ctx context.Context, argsJSON string) (string, error) {
	var a struct {
		Action string `json:"action"`
		Wind   string `json:"wind"`
		Task   string `json:"task"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", errors.New("unreadable arguments")
	}
	switch strings.TrimSpace(a.Action) {
	case "list", "":
		cards := s.windSpecialists(a.Task)
		if len(cards) == 0 {
			return "The user has no winds yet.", nil
		}
		var b strings.Builder
		b.WriteString("Winds (id · name · trade · what they learned):\n")
		for _, c := range cards {
			var noms []string
			for _, k := range c.Coneixements {
				noms = append(noms, fmt.Sprint(k["nom"]))
			}
			fmt.Fprintf(&b, "- %s · %s · %s", c.ID, c.Name, c.Role)
			if len(noms) > 0 {
				b.WriteString(" · " + strings.Join(noms, ", "))
			}
			if a.Task != "" {
				fmt.Fprintf(&b, " · match %.1f", c.Score)
			}
			b.WriteByte('\n')
		}
		return strings.TrimRight(b.String(), "\n"), nil
	case "all":
		// Independent perspectives for an explicitly requested consultation.
		task := strings.TrimSpace(a.Task)
		if task == "" {
			return "", errors.New("task is required")
		}
		ws := mustWinds(s)
		if len(ws) == 0 {
			return "The user has no winds yet.", nil
		}
		type answer struct {
			wind companion.Wind
			text string
			err  error
		}
		results := make([]answer, len(ws))
		var wg sync.WaitGroup
		slots := make(chan struct{}, 3)
		for i, w := range ws {
			wg.Add(1)
			go func(i int, w companion.Wind) {
				defer wg.Done()
				select {
				case slots <- struct{}{}:
					defer func() { <-slots }()
				case <-ctx.Done():
					results[i] = answer{wind: w, err: ctx.Err()}
					return
				}
				text, _, err := s.windAskOnce(ctx, w, task+"\n\nGive one useful observation from your specialty. If your specialty adds nothing, reply with exactly -.")
				results[i] = answer{wind: w, text: text, err: err}
			}(i, w)
		}
		wg.Wait()
		var b strings.Builder
		b.WriteString("Answers from the winds:\n")
		for _, r := range results {
			if r.err == nil && (r.text == "" || r.text == "-") {
				continue
			}
			b.WriteString("\n=== " + r.wind.Name + " (" + r.wind.ID + ") ===\n")
			if r.err != nil {
				fmt.Fprintf(&b, "ERROR: %v\n", r.err)
				continue
			}
			b.WriteString(r.text + "\n")
		}
		return strings.TrimRight(b.String(), "\n"), nil
	case "ask":
		task := strings.TrimSpace(a.Task)
		if task == "" {
			return "", errors.New("task is required")
		}
		var wind companion.Wind
		if ref := strings.TrimSpace(a.Wind); ref != "" {
			w, ok := s.winds.ByRef(ref)
			if !ok {
				if found := windsNamedIn(mustWinds(s), ref); len(found) == 1 {
					w, ok = found[0], true
				}
			}
			if !ok {
				return "", fmt.Errorf("no wind called %q (use action list)", ref)
			}
			wind = w
		} else {
			cards := s.windSpecialists(task)
			if len(cards) == 0 || cards[0].Score <= 0 {
				return "", errors.New("no wind is a specialist in this task; do it yourself or name a wind (action list shows them)")
			}
			wind, _ = s.winds.ByID(cards[0].ID)
		}
		reply, folder, err := s.windDelegate(ctx, wind, task)
		if err != nil {
			return "", err
		}
		out := wind.Name + " (" + wind.ID + ") answered:\n" + reply
		if folder != "" {
			out += "\n\nWorking folder: " + folder
		}
		return out, nil
	}
	return "", fmt.Errorf("unknown action %q (list, ask, all)", a.Action)
}

func mustWinds(s *Server) []companion.Wind {
	w, _ := s.winds.List()
	return withoutGregal(w)
}

// windAskOnce pregunta a un vent sense tocar la seva conversa: la consulta
// de l'agent (acció all) és puntual i no ha de deixar rastre al fil del
// vent ni consumir-ne l'historial. Cada vent contesta només amb el seu
// ofici i la seva memòria de coneixements.
func (s *Server) windAskOnce(parent context.Context, wind companion.Wind, task string) (string, string, error) {
	ctx, cancel := context.WithTimeout(parent, windTurnTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	// An inherited interactive scope must not let a consultation mutate files.
	ctx = s.readOnlyWindCtx(ctx)
	msg := "[Question from the main agent] " + task
	start := time.Now()
	s.mu.Lock()
	_, logRole := s.windRoleRef()
	s.mu.Unlock()
	tl := startTurnLog("wind-ask", wind.ID, logRole)
	res, err := windLoopStream(s, agentSkills(tl.withUsage(ctx), s, wind.ID), windChatBrief(s, wind, nil, msg), nil)
	tl.finish(err, tornAturat(err), res.parcial, &res.temps)
	if err != nil {
		return "", "", errors.New(redactError(err))
	}
	log.Printf("vents consulta %s: %.1fs (%s)", wind.ID, time.Since(start).Seconds(), res.temps)
	reply := stripQuoteTrim(res.text)
	if res.parcial && esRespostaMecanica(reply) {
		reply = fraseTornAturat(res.motiu, s.cfg.Lang())
	}
	ws, _ := s.ventWorkspace()
	return reply, ws, nil
}

// windDelegate fa fer una feina a un vent (com si la persona li ho hagués
// demanat al seu xat) i en torna la resposta. L'encàrrec i la resposta
// queden a la conversa del vent, així la persona ho pot seguir des del mòbil.
func (s *Server) windDelegate(parent context.Context, wind companion.Wind, task string) (string, string, error) {
	hist := windHistory(s, wind.ID)
	turn := windDirectTurn(s, wind.ID)
	turn.Lock()
	defer turn.Unlock()
	if err := parent.Err(); err != nil {
		return "", "", err
	}
	hist.mu.RLock()
	if hist.deleted && hist.name != "company-"+wind.ID {
		hist.mu.RUnlock()
		return "", "", errors.New("session no longer exists")
	}
	history := append([]llm.Message(nil), hist.messages...)
	gen := hist.gen
	hist.mu.RUnlock()
	previous := history
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	ctx, cancel := context.WithTimeout(context.Background(), windTurnTimeout)
	defer cancel()
	stop := context.AfterFunc(parent, cancel) // aturar el torn de dalt atura el vent
	defer stop()
	if b := linkBaseOf(parent); b != "" {
		ctx = context.WithValue(ctx, linkBaseKey{}, b)
	}
	ctx, doneTurn := s.beginWindTurn("wind", wind.ID, ctx)
	defer doneTurn()
	ctx = s.windDocsCtx(s.interactiveWindCtx(ctx))
	msg := "[Task handed over by the main agent] " + task
	start := time.Now()
	s.mu.Lock()
	_, logRole := s.windRoleRef()
	s.mu.Unlock()
	tl := startTurnLog("wind-task", wind.ID, logRole)
	res, err := windLoopStream(s, agentSkills(tl.withUsage(ctx), s, wind.ID), windChatBrief(s, wind, previous, msg), nil)
	tl.finish(err, tornAturat(err), res.parcial, &res.temps)
	if err != nil {
		return "", "", errors.New(redactError(err))
	}
	log.Printf("vents encàrrec %s: %.1fs (%s)", wind.ID, time.Since(start).Seconds(), res.temps)
	reply := stripQuoteTrim(res.text)
	if res.parcial && esRespostaMecanica(reply) {
		reply = fraseTornAturat(res.motiu, s.cfg.Lang())
	}
	reply += s.windDocLinks(start, reply)
	history = append(history,
		llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()},
		llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
	hist.mu.Lock()
	if hist.gen == gen {
		if err := saveWindHistoryTo(s, wind, hist.name, history); err != nil {
			hist.mu.Unlock()
			return "", "", err
		}
		hist.messages = podaHistorialVent(history)
		hist.gen++
	}
	hist.mu.Unlock()
	s.startWindLearn(wind, msg, reply)
	s.startWindLessons(wind, msg, reply, res.hist)
	ws, _ := s.ventWorkspace()
	return reply, ws, nil
}

// agentSkills posa els coneixements del vent al context del torn.
func agentSkills(ctx context.Context, s *Server, windID string) context.Context {
	return agent.WithSkillsDirs(ctx, s.windSkillsDir(windID))
}
