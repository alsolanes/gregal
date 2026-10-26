package web

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
	"unicode"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Router de rotllana: amb una crida barata (el mateix model dels vents)
// tria quins membres responen (1-3, per rellevància) o cap. Abans parlava
// tothom per torn mecànic; ara només intervé qui hi té res a dir:
// l'expert (coneixement), qui en sap pel seu rol, o per afinitat — i si
// no hi ha ni afinitat, silenci.

// rotllanaRouteLoop és la via del router; els tests la substitueixen.
var rotllanaRouteLoop = func(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// Només cal un JSON curt, i sempre el mateix per al mateix missatge
	// (temperatura 0). El límit de tokens és el del rol: amb models que
	// raonen, un límit petit es menja el JSON i força el model de recanvi.
	out, _, err := client.ChatFO(llm.WithSideCall(ctx2), cfg.PrimTarget(p, role), cfg.FallbackTarget(role), msgs, 0, role.MaxTokens, nil)
	return out, err
}

// windNamedIn torna el vent que el missatge nomena (pel nom o amb @), si n'és
// només un: «Garbí, i el Madrid?» ja diu qui ha de respondre.
func windNamedIn(members []companion.Wind, msg string) (companion.Wind, bool) {
	low := strings.ToLower(msg)
	var found []companion.Wind
	for _, w := range members {
		name := strings.ToLower(strings.TrimSpace(w.Name))
		if name == "" {
			continue
		}
		for i := strings.Index(low, name); i >= 0; {
			end := i + len(name)
			before, after := ' ', ' '
			if i > 0 {
				before = []rune(low[:i])[len([]rune(low[:i]))-1]
			}
			if end < len(low) {
				after = []rune(low[end:])[0]
			}
			if !unicode.IsLetter(before) && !unicode.IsLetter(after) {
				found = append(found, w)
				break
			}
			next := strings.Index(low[end:], name)
			if next < 0 {
				break
			}
			i = end + next
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return companion.Wind{}, false
}

// routeRotllanaSpeakers torna els vents que han de parlar, en ordre.
// Buit = silenci (ningú hi té res a dir). Amb `wanted` o si el router
// falla, cau al torn clàssic d'un sol vent.
func (s *Server) routeRotllanaSpeakers(ctx context.Context, rot Rotllana, wanted, msg string, tail []llm.Message) []companion.Wind {
	members := make([]companion.Wind, 0, len(rot.Members))
	for _, id := range rot.Members {
		if w, ok := s.winds.ByID(id); ok {
			members = append(members, w)
		}
	}
	if len(members) == 0 {
		return nil
	}
	if wanted != "" {
		for _, w := range members {
			if w.ID == wanted {
				return []companion.Wind{w}
			}
		}
		return nil
	}
	if len(members) == 1 {
		return members
	}
	if w, ok := windNamedIn(members, msg); ok {
		return []companion.Wind{w}
	}
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	var recent []string
	for _, m := range tail {
		if m.Role != "assistant" {
			continue
		}
		if m.SpeakerID != "" {
			recent = append(recent, m.SpeakerID)
		}
	}
	var b strings.Builder
	b.WriteString("You route a group chat. The user wrote:\n" + msg + "\n\nMembers (only their descriptions and what they remember — never their full knowledge):\n")
	for _, w := range members {
		b.WriteString("- " + w.ID + " | " + w.Name + " | rol: " + w.Role + "\n")
		var descs []string
		for _, sk := range s.windSkills(w.ID) {
			descs = append(descs, sk.Descripcio)
		}
		if len(descs) > 0 {
			d := strings.Join(descs, "; ")
			if len(d) > 300 {
				d = d[:300]
			}
			b.WriteString("  sap: " + d + "\n")
		}
		if notes := companion.WindMemoryNotes(s.windStateDir(), w.ID); len(notes) > 0 {
			m := strings.Join(notes, "; ")
			if len(m) > 300 {
				m = m[:300]
			}
			b.WriteString("  recorda: " + m + "\n")
		}
	}
	if len(recent) > 0 {
		b.WriteString("Recently spoke (avoid repeating them unless directly asked): " + strings.Join(recent, ", ") + "\n")
	}
	b.WriteString("\nPick who should answer, most relevant first: usually ONE member — the one whose knowledge matches, else the closest role/affinity. Add a second member only when they would bring something clearly different (another angle, a check), never a third. Each extra voice makes the user wait longer. If nobody has anything to add, pick none.\n")
	b.WriteString("Reply ONLY JSON like {\"parlen\":[\"id\",\"id\"]} with existing ids. [] means silence.")
	out, err := rotllanaRouteLoop(s, ctx, []llm.Message{{Role: "user", Content: b.String()}})
	if err != nil {
		log.Printf("vents tria: %s; parla qui fa més que no parla", redactError(err))
		return classicRotllanaSpeaker(members, tail)
	}
	var parsed struct {
		Parlen []string `json:"parlen"`
	}
	dec := json.NewDecoder(strings.NewReader(out))
	// El model de vegades embolcalla; busca el primer { ... }.
	raw := out
	if i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}"); i >= 0 && j > i {
		raw = raw[i : j+1]
	}
	dec = json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&parsed); err != nil {
		log.Printf("vents tria: resposta sense JSON (%d caràcters); parla qui fa més que no parla", len(out))
		return classicRotllanaSpeaker(members, tail)
	}
	byID := map[string]companion.Wind{}
	for _, w := range members {
		byID[w.ID] = w
	}
	var speakers []companion.Wind
	seen := map[string]bool{}
	for _, id := range parsed.Parlen {
		if w, ok := byID[strings.TrimSpace(id)]; ok && !seen[w.ID] {
			seen[w.ID] = true
			speakers = append(speakers, w)
		}
		if len(speakers) == 2 { // cada veu és una espera més: dues com a màxim
			break
		}
	}
	// Llista buida vàlida = silenci. Parse sense ids vàlids = error del
	// model, no silenci: cau al clàssic.
	if len(parsed.Parlen) > 0 && len(speakers) == 0 {
		return classicRotllanaSpeaker(members, tail)
	}
	return speakers
}

// classicRotllanaSpeaker és el torn d'abans: el que menys ha parlat.
func classicRotllanaSpeaker(members []companion.Wind, tail []llm.Message) []companion.Wind {
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	counts := map[string]int{}
	for _, m := range tail {
		if m.Role != "assistant" {
			continue
		}
		for _, w := range members {
			if m.SpeakerID == w.ID || (m.SpeakerID == "" && strings.HasPrefix(m.Content, w.Name+":")) {
				counts[w.ID]++
			}
		}
	}
	best := members[0]
	for _, w := range members[1:] {
		if counts[w.ID] < counts[best.ID] {
			best = w
		}
	}
	return []companion.Wind{best}
}
