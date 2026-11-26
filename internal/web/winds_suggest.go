package web

import (
	"context"
	"encoding/json"
	"log"
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
//	POST /api/winds/suggest  {description} → {name, role, persona, style, color, kit}
//
// La persona i la cara són una proposta: l'usuari les veu al creador i hi
// pot tocar el que vulgui abans de crear el vent.

// windSuggestSystem compon el system de la proposta: dóna veu al vent.
func windSuggestSystem(name, role string, collective ...string) string {
	var b strings.Builder
	plural := ""
	if len(collective) > 0 {
		plural = strings.TrimSpace(collective[0])
	}
	if isWindCollective(plural) {
		b.WriteString("You write the persona of a wind companion in an app called Gregal. ")
		b.WriteString("The user gives you a wind name (winds and breezes, e.g. Tramuntana, Xaloc, Garbinada, Terral, Ventolina…) and what it watches over. ")
	} else {
		// La persona ha triat com es diuen els seus ajudants: la veu hi encaixa.
		b.WriteString("You write the persona of a helper companion in an app called Gregal. The user calls these helpers \"" + plural + "\": give it a personality that fits that idea. ")
		b.WriteString("The user gives you its name and what it watches over. ")
	}
	b.WriteString("Reply ONLY with a compact JSON object, no prose, no markdown fences, with two keys:\n")
	b.WriteString(`{"persona": "<3-5 sentences in Catalan: who this wind is, how it talks (warm, curt, playful or calm — pick a coherent voice), what it cares about>", "style": "<exactly 3 letters: eyes+mouth+shape>"}`)
	b.WriteString("\nstyle letters: eyes p=punts, b=badalls, e=estrellats, m=mig tancats; mouth s=somriure, o=oberta, n=neutra, t=llengua; shape o=bola, n=núvol, s=espiral, g=gota, u=sol, w=onada. ")
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
		// Sense nom, una descripció basta: la IA proposa el vent sencer.
		Description string `json:"description"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if desc := strings.TrimSpace(req.Description); name == "" && desc != "" {
		s.suggestWindFromDescription(w, desc)
		return
	}
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
		{Role: "system", Content: windSuggestSystem(name, role, s.loadCollective().Plural)},
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

// windDescribeSystem: d'una frase, el vent sencer. El nom segueix el
// col·lectiu i evita els que ja es fan servir; el rol diu l'ofici amb una
// paraula que la regla rol→kit (winds_kit.go) reconeix.
func windDescribeSystem(plural, nameRule string, taken []string) string {
	var b strings.Builder
	if isWindCollective(plural) {
		b.WriteString("You invent a wind companion for an app called Gregal. ")
	} else {
		b.WriteString("You invent a helper companion for an app called Gregal. The user calls these helpers \"" + plural + "\": give it a name and a personality that fit that idea. ")
	}
	b.WriteString("The user describes in one sentence the helper they want; you propose the rest. ")
	b.WriteString("Reply ONLY with a compact JSON object, no prose, no markdown fences, with five keys:\n")
	b.WriteString(`{"name": "<` + nameRule + `>", "role": "<2-5 words in Catalan>", "persona": "<3-5 sentences in Catalan: who it is, how it talks (warm, curt, playful or calm — pick a coherent voice), what it cares about>", "style": "<exactly 3 letters: eyes+mouth+shape>", "color": "<a soft pastel colour as #rrggbb that suits it>"}`)
	if len(taken) > 0 {
		b.WriteString("\nThese names are taken, never use them: " + strings.Join(taken, ", ") + ".")
	}
	b.WriteString("\nrole says what it helps with and names its field with a plain word when one fits: cuina, música, viatges, esport, salut, finances, llibres, cinema, el temps, seguretat, programació, recerca, organització, compres.")
	b.WriteString("\nstyle letters: eyes p=punts, b=badalls, e=estrellats, m=mig tancats; mouth s=somriure, o=oberta, n=neutra, t=llengua; shape o=bola, n=núvol, s=espiral, g=gota, u=sol, w=onada. ")
	b.WriteString("Pick the style that matches the persona you wrote. Write the persona in Catalan, playful but never fake. Never invent facts about the user.")
	return b.String()
}

// suggestWindFromDescription respon {name, role, persona, style, color, kit}:
// tot és una proposta que l'usuari revisa al creador abans de crear el vent.
func (s *Server) suggestWindFromDescription(w http.ResponseWriter, desc string) {
	p, roleRef, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "no side role available", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()

	used, usedColors := map[string]bool{}, map[string]bool{}
	var taken []string
	if s.winds != nil {
		ws, err := s.winds.List()
		if err != nil {
			log.Printf("wind suggest: list winds: %v", err)
		}
		for _, x := range ws {
			used[strings.ToLower(strings.TrimSpace(x.Name))] = true
			usedColors[strings.ToUpper(x.Color)] = true
			taken = append(taken, x.Name)
		}
	}
	collective := s.loadCollective()
	msgs := []llm.Message{
		{Role: "system", Content: windDescribeSystem(collective.Plural, s.collectiveNameRule("Catalan"), taken)},
		{Role: "user", Content: "Description: " + oneLine(desc, 600)},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := windSuggestLoop(s, ctx, client, cfg, p, roleRef, msgs)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	var parsed struct {
		Name  string `json:"name"`
		Role  string `json:"role"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal([]byte(firstJSONObject(out)), &parsed); err != nil {
		log.Printf("wind suggest: unparsable reply, using defaults")
	}
	name := cleanCrewName(parsed.Name)
	if name == "" || used[strings.ToLower(name)] {
		name = nextCrewName(used, s.freshNames(s.collectiveNames(ctx, collective, "ca"))...)
	}
	// Sense rol, la descripció mateixa: la regla rol→kit hi troba l'ofici.
	role := oneLine(parsed.Role, 60)
	if role == "" {
		role = oneLine(desc, 60)
	}
	color := strings.ToLower(strings.TrimSpace(parsed.Color))
	if !transferColorPattern.MatchString(color) {
		color = strings.ToLower(nextCrewColor(usedColors))
	}
	persona, style := parseWindSuggest(out, name)
	writeJSON(w, map[string]string{"name": name, "role": role, "persona": persona, "style": style, "color": color, "kit": inferKit(role)})
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
	shape := map[string]string{"o": "o", "n": "n", "s": "s", "g": "g", "u": "u", "w": "w"}
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
