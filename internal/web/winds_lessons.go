package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/skills"
)

// Lliçons de feina: el que un vent aprèn fent.
//
// El cervell (windbrain_web.go) només desa fets del tema amb font citada, i
// l'estudi (winds_expert.go) es fa un cop. Però el que fa bo un especialista
// és el que passa a la feina: quina ordre funciona en aquest servidor, quin
// error ha sortit i com s'ha resolt, quin pas no es pot saltar. Després de
// cada torn amb eines, una crida petita al model lateral llegeix el rastre
// (eines, errors, resultat) i escriu 0-3 lliçons noves al coneixement
// `llicons-de-feina` del vent. Les últimes van directament al prompt dels
// torns següents (no cal que el vent obri la skill per seguir-les).

const (
	windLessonsSkill   = "llicons-de-feina"
	windLessonsMax     = 40
	windLessonsInBrief = 12
)

var windLessonsMu sync.Map // windID → *sync.Mutex

func windLessonsLock(id string) *sync.Mutex {
	m, _ := windLessonsMu.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// windLessons torna les lliçons desades (línies «- …»), les més noves al final.
func (s *Server) windLessons(windID string) []string {
	for _, sk := range s.windSkills(windID) {
		if sk.Nom == windLessonsSkill {
			return lessonLines(sk.Cos)
		}
	}
	return nil
}

func lessonLines(body string) []string {
	var out []string
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- ") && len(l) > 4 {
			out = append(out, l)
		}
	}
	return out
}

// windLessonsBrief va al prompt del torn: les últimes lliçons, per seguir-les.
func (s *Server) windLessonsBrief(windID string) string {
	ls := s.windLessons(windID)
	if len(ls) == 0 {
		return ""
	}
	if len(ls) > windLessonsInBrief {
		ls = ls[len(ls)-windLessonsInBrief:]
	}
	return "\n\nLESSONS FROM YOUR PAST JOBS ON THIS SERVER (follow them; they come from real errors and fixes):\n" + strings.Join(ls, "\n")
}

// addWindLessons afegeix lliçons noves (sense repetides) i en guarda les
// últimes windLessonsMax.
func (s *Server) addWindLessons(windID string, nous []string) (int, error) {
	mu := windLessonsLock(windID)
	mu.Lock()
	defer mu.Unlock()
	have := s.windLessons(windID)
	seen := map[string]bool{}
	for _, l := range have {
		seen[lessonKey(l)] = true
	}
	added := 0
	for _, l := range nous {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "- ") {
			l = "- " + strings.TrimLeft(l, "-*• ")
		}
		if len([]rune(l)) > 260 || len(l) < 8 {
			continue
		}
		k := lessonKey(l)
		if seen[k] {
			continue
		}
		seen[k] = true
		have = append(have, l)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	if len(have) > windLessonsMax {
		have = have[len(have)-windLessonsMax:]
	}
	err := s.saveWindSkill(windID, skills.Skill{
		Nom:        windLessonsSkill,
		Titol:      "Lliçons de feina",
		Descripcio: "Lessons from my past jobs on this server: commands that work here, errors I hit and how I fixed them. Read before building.",
		Cos:        "# Lliçons de feina\n\n" + strings.Join(have, "\n"),
	})
	return added, err
}

func lessonKey(l string) string {
	return strings.Join(paraulesDe(strings.TrimPrefix(l, "- ")), " ")
}

// lessonTrace resumeix el rastre d'eines d'un torn: què s'ha cridat, què ha
// fallat i què ha funcionat. Buit si el torn no ha fet servir eines.
func lessonTrace(hist []llm.Message) string {
	args := map[string]string{}
	var lines []string
	for _, m := range hist {
		if m.Role == "assistant" {
			for _, c := range m.ToolCalls {
				args[c.ID] = c.Function.Name + " " + compactArgs(c.Function.Arguments)
			}
			continue
		}
		if m.Role != "tool" {
			continue
		}
		call := args[m.ToolCallID]
		if call == "" {
			call = m.Name
		}
		out := strings.TrimSpace(m.Content)
		mark := "ok"
		low := strings.ToLower(out)
		if strings.HasPrefix(out, "ERROR") || strings.Contains(low, "requires approval") || strings.Contains(low, "denied") || strings.Contains(low, "denegat") || strings.Contains(low, "exit status") || strings.Contains(low, "not found") || strings.Contains(low, "traceback") || strings.Contains(low, "error:") {
			mark = "FAILED"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s → %s", mark, cut(call, 200), cut(strings.ReplaceAll(out, "\n", " ⏎ "), 260)))
	}
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > 28 {
		lines = append(lines[:6], lines[len(lines)-22:]...)
	}
	return strings.Join(lines, "\n")
}

func compactArgs(raw string) string {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return cut(raw, 160)
	}
	// El contingut sencer d'un fitxer no ensenya res: només on.
	delete(m, "content")
	delete(m, "patch")
	delete(m, "new_string")
	delete(m, "old_string")
	b, _ := json.Marshal(m)
	return string(b)
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// startWindLessons aprèn del torn en segon pla (mai no fa fallar el xat).
func (s *Server) startWindLessons(w companion.Wind, task, reply string, hist []llm.Message) {
	trace := lessonTrace(hist)
	if trace == "" {
		return
	}
	goBackground(func() { s.windLessonsLearn(w, task, reply, trace) })
}

// windLessonsLoop és la crida al model lateral; els tests la substitueixen.
var windLessonsLoop = func(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	p, role, ok := s.sideRoleRef()
	if !ok {
		return "", fmt.Errorf("no side model")
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	return windLearnLoop(s, ctx, client, cfg, p, role, msgs)
}

func (s *Server) windLessonsLearn(w companion.Wind, task, reply, trace string) {
	defer func() { _ = recover() }()
	known := s.windLessons(w.ID)
	var b strings.Builder
	b.WriteString("You keep the work playbook of " + w.Name + ", a specialist helper whose trade is: " + w.Role + ".\n")
	b.WriteString("Below is the trace of a job it just did on this server (tool calls, what failed, what worked) and its final answer.\n")
	b.WriteString("Write 0-3 NEW lessons that will make the next similar job faster or avoid a repeated error: facts about this environment (which commands and tools exist and work here, which are blocked and the alternative that worked), fixes to errors that happened, steps that proved necessary, patterns that worked well.\n")
	b.WriteString("Rules: each lesson is ONE line starting with '- ', imperative and concrete (a command, a flag, a file name), at most 200 characters, in the user's language (Catalan if unsure). Nothing specific to this one project's content. No lesson that repeats a known one. If there is nothing new worth keeping, reply exactly NONE.\n")
	if len(known) > 0 {
		b.WriteString("\nKnown lessons (do NOT repeat):\n" + strings.Join(known, "\n") + "\n")
	}
	b.WriteString("\nTask: " + cut(task, 600) + "\n\nTrace:\n" + trace + "\n\nFinal answer: " + cut(reply, 800))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := windLessonsLoop(s, ctx, []llm.Message{{Role: "user", Content: b.String()}})
	if err != nil || strings.Contains(strings.ToUpper(out), "NONE") && !strings.Contains(out, "\n- ") && !strings.HasPrefix(strings.TrimSpace(out), "- ") {
		return
	}
	var nous []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- ") {
			nous = append(nous, l)
			if len(nous) == 3 {
				break
			}
		}
	}
	_, _ = s.addWindLessons(w.ID, nous)
}
