package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
)

// rotllanaMaxVoices és el topall de veus per missatge quan el router tria.
const rotllanaMaxVoices = 4

// rotllanaRouteModel és la crida petita que tria qui parla; els tests la
// substitueixen. Sense model configurat torna error i es fa servir la regla
// local (puntuació d'ofici + participació).
var rotllanaRouteModel = func(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	if client == nil || cfg == nil {
		return "", errConst("no model configured")
	}
	prim, fb := cfg.PrimTarget(p, role), cfg.FallbackTarget(role)
	if _, sr, sp, ok := agent.SmallRole(cfg); ok {
		if small := cfg.PrimTarget(sp, sr); small != prim {
			orig := prim
			prim, fb, role = small, &orig, sr
		}
	}
	ctx, cancel := context.WithTimeout(llm.WithThink(ctx, "no"), 12*time.Second)
	defer cancel()
	out, _, err := client.ChatFO(ctx, prim, fb, msgs, 0, 160, nil)
	return strings.TrimSpace(out), err
}

// modelRotllanaSpeakers demana a un model petit qui ha de respondre. La regla
// lèxica (paraules del missatge contra l'ofici) falla amb «quin llenguatge
// per a una API?» i, quan no troba res, es quedava amb l'últim que havia
// parlat: un sol vent acabava responent-ho tot. Aquí es decideix pel sentit.
func (s *Server) modelRotllanaSpeakers(ctx context.Context, members []companion.Wind, msg string, tail []llm.Message) ([]companion.Wind, bool) {
	var roster strings.Builder
	for _, w := range members {
		role := strings.TrimSpace(w.Role)
		if role == "" {
			role = "generalist"
		}
		fmt.Fprintf(&roster, "- %s · %s · %s\n", w.ID, w.Name, role)
	}
	last := "(nobody yet)"
	if w, ok := lastRotllanaSpeaker(members, tail); ok {
		last = w.ID
	}
	system := "You route messages in a group chat of AI assistants (\"winds\") with one human. Pick who answers the human's LAST message.\n" +
		"Members (id · name · trade):\n" + roster.String() +
		"Rules:\n" +
		"- Pick the member whose trade best fits the topic, even if somebody else spoke last. Never pick someone just because they spoke last or least.\n" +
		"- If the human addresses the group in plural (\"vosaltres\", \"què en penseu\", \"opineu\", \"you all\", \"everyone\", \"cadascú\") or asks for opinions or a debate, pick several members (all that have something relevant to say, at most " + fmt.Sprint(rotllanaMaxVoices) + ").\n" +
		"- If the message is a short follow-up that only makes sense with the previous answer (\"sí\", \"endavant\", \"i això?\", \"fes-ho\"), pick the member who spoke last: " + last + ".\n" +
		"- If nobody's trade fits, pick the most generalist member.\n" +
		"Answer ONLY with JSON: {\"speakers\":[\"id\", ...]} using ids from the list, in speaking order."
	user := "Recent conversation:\n" + agent.TranscriptDigestSpeakers(tail, 1800) + "\n\nHuman's last message:\n" + msg
	raw, err := rotllanaRouteModel(s, ctx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}})
	if err != nil {
		return nil, false
	}
	var out struct {
		Speakers []string `json:"speakers"`
	}
	obj := firstJSONObject(raw)
	if obj == "" || json.Unmarshal([]byte(obj), &out) != nil {
		return nil, false
	}
	byID := map[string]companion.Wind{}
	for _, w := range members {
		byID[w.ID] = w
	}
	seen := map[string]bool{}
	var picked []companion.Wind
	for _, id := range out.Speakers {
		id = strings.TrimSpace(id)
		w, ok := byID[id]
		if !ok {
			// El model pot haver escrit el nom en lloc de l'id.
			for _, m := range members {
				if strings.EqualFold(strings.TrimSpace(m.Name), id) {
					w, ok = m, true
					break
				}
			}
		}
		if !ok || seen[w.ID] {
			continue
		}
		seen[w.ID] = true
		picked = append(picked, w)
		if len(picked) == rotllanaMaxVoices {
			break
		}
	}
	return picked, len(picked) > 0
}
