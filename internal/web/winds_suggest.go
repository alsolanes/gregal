package web

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
	"net/http"
)

// Inventar un vent: l'usuari posa nom i rol, i la IA li escriu la persona
// (com parla, què li importa) i proposa la cara (ulls, boca, forma).
//
//	POST /api/winds/suggest  {name, role} → {persona, style}
//
// La persona i la cara són una proposta: l'usuari les veu al creador i hi
// pot tocar el que vulgui abans de crear el vent.

// windSuggestSystem compon el system de la proposta: dóna veu al vent.
func windSuggestSystem(name, role string) string {
	var b strings.Builder
	b.WriteString("You write the persona of a wind companion in an app called Gregal. ")
	b.WriteString("The user gives you a wind name (winds of the Catalan compass rose: Tramuntana, Xaloc, Llevant, Gregal, Ponent, Mestral, Llebeig, Marinada…) and what it watches over. ")
	b.WriteString("Reply ONLY with a compact JSON object, no prose, no markdown fences, with two keys:\n")
	b.WriteString(`{"persona": "<3-5 sentences in Catalan: who this wind is, how it talks (warm, curt, playful or calm — pick a coherent voice), what it cares about>", "style": "<exactly 3 letters: eyes+mouth+shape>"}`)
	b.WriteString("\nstyle letters: eyes p=punts, b=badalls, e=estrellats, m=mig tancats; mouth s=somriure, o=oberta, n=neutra, t=llengua; shape o=bola, n=núvol, s=espiral. ")
	b.WriteString("Pick the style that matches the persona you wrote (a playful wind gets a tongue or open mouth; a calm night wind gets sleepy eyes). ")
	b.WriteString("Write the persona in Catalan, playful but never fake. Never invent facts about the user.")
	return b.String()
}

// handleWindSuggest proposa persona i cara per a un vent nou.
func (s *Server) handleWindSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "nom obligatori", http.StatusBadRequest)
		return
	}
	role := strings.TrimSpace(req.Role)

	p, roleRef, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "no side role available", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()

	msgs := []llm.Message{
		{Role: "system", Content: windSuggestSystem(name, role)},
		{Role: "user", Content: "Wind name: " + name + "\nWhat it watches: " + firstNonEmpty(role, "company de tot un poc")},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := windSuggestLoop(s, ctx, client, cfg, p, roleRef, msgs)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	persona, style := parseWindSuggest(out, name)
	writeJSON(w, map[string]string{"persona": persona, "style": style})
}

// windSuggestLoop és la via de generació; els tests la substitueixen.
var windSuggestLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
	out, _, err := client.ChatFO(ctx, cfg.PrimTarget(p, role), cfg.FallbackTarget(role), msgs, role.Temperature, role.MaxTokens, nil)
	return out, err
}

// parseWindSuggest tolera respones amb prosa o tanques de codi al voltant.
func parseWindSuggest(out, name string) (persona, style string) {
	raw := strings.TrimSpace(out)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	persona = companion.DefaultWindPersona(name)
	style = "pso"
	var parsed struct {
		Persona string `json:"persona"`
		Style   string `json:"style"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		if strings.TrimSpace(parsed.Persona) != "" {
			persona = strings.TrimSpace(parsed.Persona)
		}
		if s := strings.TrimSpace(parsed.Style); len(s) >= 3 {
			style = strings.ToLower(s[:3])
		}
	}
	// Valida les lletres: ulls/boca/forma conegudes o defecte per peça.
	style = normalizeWindStyle(style)
	return persona, style
}

func normalizeWindStyle(style string) string {
	eyes := map[string]string{"p": "p", "b": "b", "e": "e", "m": "m"}
	mouth := map[string]string{"s": "s", "o": "o", "n": "n", "t": "t"}
	shape := map[string]string{"o": "o", "n": "n", "s": "s"}
	get := func(m map[string]string, ch byte, def string) string {
		if v, ok := m[strings.ToLower(string(ch))]; ok {
			return v
		}
		return def
	}
	if len(style) < 3 {
		return "pso"
	}
	return get(eyes, style[0], "p") + get(mouth, style[1], "s") + get(shape, style[2], "o")
}
