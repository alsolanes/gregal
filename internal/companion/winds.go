package companion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Els companions de Gregal són vents: Tramuntana, Xaloc, Mistral… Com el
// mateix Gregal. Cada vent té nom, color, persona i (opcionalment) una feina
// proactiva programada (job) que fa cerques/tasques periòdiques i avisa amb
// notificació. No hi ha cap "Dot": aquí mana el mestral de ponent.

// Wind és un companion personal, desat en disc.
type Wind struct {
	ID        string `json:"id"`      // id estable (slug del nom)
	Name      string `json:"name"`    // nom de vent: Tramuntana, Xaloc…
	Color     string `json:"color"`   // hex triat per l'usuari, p.ex. #FF9F00
	Persona   string `json:"persona"` // veu: com parla i què li importa
	Role      string `json:"role"`    // subtítol curt de la UI: "música", "notícies"…
	Style     string `json:"style"`   // peces de la cara: ulls/boca/forma (p.ex. "pso")
	Squad     string `json:"squad,omitempty"`
	Specialty string `json:"specialty,omitempty"`
}

// WindStore desa els vents al costat del config (winds.json), com els goals.
type WindStore struct {
	dir string
	mu  sync.Mutex
}

func NewWindStore(dir string) *WindStore { return &WindStore{dir: dir} }

func (ws *WindStore) path() string { return filepath.Join(ws.dir, "winds.json") }

// PathForTest exposa el directori dels vents (per a les proves).
func (ws *WindStore) PathForTest() string { return ws.dir }

type windDisc struct {
	Winds []Wind `json:"winds"`
}

func (ws *WindStore) List() ([]Wind, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	raw, err := os.ReadFile(ws.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var d windDisc
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return d.Winds, nil
}

func (ws *WindStore) Save(w Wind) (Wind, error) {
	if strings.TrimSpace(w.Name) == "" {
		return Wind{}, fmt.Errorf("el vent necessita un nom")
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	raw, err := os.ReadFile(ws.path())
	if err != nil && !os.IsNotExist(err) {
		return Wind{}, err
	}
	var d windDisc
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			return Wind{}, err
		}
	}
	explicit := w.ID != ""
	if !explicit {
		w.ID = slug(strings.TrimSpace(w.Name))
	}
	index := -1
	for i := range d.Winds {
		if d.Winds[i].ID == w.ID {
			index = i
			break
		}
	}
	if explicit && index < 0 {
		return Wind{}, fmt.Errorf("unknown wind")
	}
	if strings.TrimSpace(w.Persona) == "" {
		if index >= 0 && strings.TrimSpace(d.Winds[index].Persona) != "" {
			w.Persona = d.Winds[index].Persona
		} else {
			w.Persona = DefaultWindPersona(w.Name)
		}
	}
	if !strings.HasPrefix(w.Color, "#") || len(w.Color) != 7 {
		w.Color = "#FF9F00"
	}
	if index >= 0 {
		// Older Android/desktop editors do not send squad metadata.
		if w.Squad == "" {
			w.Squad = d.Winds[index].Squad
		}
		if w.Specialty == "" {
			w.Specialty = d.Winds[index].Specialty
		}
		d.Winds[index] = w
	} else {
		d.Winds = append(d.Winds, w)
	}
	if err := os.MkdirAll(ws.dir, 0o755); err != nil {
		return Wind{}, err
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	return w, os.WriteFile(ws.path(), b, 0o644)
}

func (ws *WindStore) Delete(id string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	raw, err := os.ReadFile(ws.path())
	if err != nil {
		return err
	}
	var d windDisc
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	out := make([]Wind, 0, len(d.Winds))
	for _, w := range d.Winds {
		if w.ID != id {
			out = append(out, w)
		}
	}
	b, _ := json.MarshalIndent(windDisc{Winds: out}, "", "  ")
	return os.WriteFile(ws.path(), b, 0o644)
}

// Restore inserts a known identity without deriving its ID from its display name.
// Existing identities are never overwritten (used by archives and starter squads).
func (ws *WindStore) Restore(w Wind) error {
	if w.ID == "" || slug(w.ID) != w.ID || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("invalid wind identity")
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	var d windDisc
	raw, err := os.ReadFile(ws.path())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
	}
	for _, existing := range d.Winds {
		if existing.ID == w.ID {
			return fmt.Errorf("wind already exists")
		}
	}
	d.Winds = append(d.Winds, w)
	if err := os.MkdirAll(ws.dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(ws.dir, ".winds-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), ws.path())
}

// WindByID cerca un vent per id.
func (ws *WindStore) ByID(id string) (Wind, bool) {
	list, err := ws.List()
	if err != nil {
		return Wind{}, false
	}
	for _, w := range list {
		if w.ID == id {
			return w, true
		}
	}
	return Wind{}, false
}

// ByRef troba un vent per id, nom o slug del nom: els vents reben referències
// del model i no sempre encerten l'id llarg. Torna el vent amb l'id real.
func (ws *WindStore) ByRef(ref string) (Wind, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Wind{}, false
	}
	if w, ok := ws.ByID(ref); ok {
		return w, true
	}
	list, err := ws.List()
	if err != nil {
		return Wind{}, false
	}
	for _, w := range list {
		if strings.EqualFold(strings.TrimSpace(w.Name), ref) || slug(w.Name) == slug(ref) {
			return w, true
		}
	}
	return Wind{}, false
}

// SlugWind normalitza un nom (vent o rotllana) per fer-ne id.
func SlugWind(s string) string { return slug(s) }

// slug normalitza un nom per fer-ne id: minúscules, sense accents ni espais.
func slug(s string) string {
	s = strings.ToLower(s)
	repl := map[rune]rune{'á': 'a', 'à': 'a', 'ä': 'a', 'ç': 'c', 'é': 'e', 'è': 'e', 'í': 'i', 'ï': 'i', 'ó': 'o', 'ò': 'o', 'ú': 'u', 'ü': 'u', 'ñ': 'n'}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if repl, ok := repl[r]; ok {
			r = repl
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else if len(out) > 0 && out[len(out)-1] != '-' {
			out = append(out, '-')
		}
	}
	return strings.Trim(string(out), "-")
}

// DefaultWindPersona és la veu per defecte d'un vent nou: càlida i curta.
func DefaultWindPersona(name string) string {
	return "Ets " + name + ", un vent de Gregal: un company petit i càlid. Parles curt (1-4 frases), en català, amb caliu i una punta d'entusiasme de debò — mai farciment. Coneixes la persona (la memòria t'ho diu) i saps què passa al projecte (el context t'ho diu). Si et demanen feina que toca fitxers o execucions, explica què faries i proposa llençar-la amb «Investiga» (només mirar) o «Implementa» (fer-ho, amb aprovació). Mai t'inventis dades."
}

// WindChatSystem compon el system del xat d'un vent: persona + context real
// del projecte (objectius pendents, darrera sessió) + memòria d'usuari.
func WindChatSystem(w Wind, memory, project string, goals []string, lastSessionDigest, userName string) string {
	var b strings.Builder
	b.WriteString(w.Persona)
	if n := strings.TrimSpace(userName); n != "" {
		b.WriteString("\n\nWHO YOU TALK TO:\nYou chat with one person: " + n + ". Use their name only when it helps, and never anyone else's.")
	} else {
		b.WriteString("\n\nWHO YOU TALK TO:\nYou chat with one person whose name you do NOT know. If a name would help, ask them; NEVER invent a name and NEVER borrow the name of another wind or agent.")
	}
	b.WriteString("\n\nHOW YOU TALK:\n- Short replies (1-4 sentences), warm, direct, in the user's language (Catalan if unsure).\n- You may be playful, never fake-enthusiastic. Personality over politeness.\n- Warm and respectful always: no swearing, no rudeness — unless your persona explicitly asks for it.\n- Never invent facts: what you know is below; anything else, ask or admit it.\n- You can search the web (web_search, web_fetch) and open pages in a real browser (browser: open/goto/read/screenshot only — look, never click or type). When a page only renders with JavaScript or web_fetch fails, open it with browser and read it. Cite sources inline like [title](url).")
	b.WriteString("\n- You can also find IMAGES with web_images and embed the useful ones in your reply as ![title](image-url) on their own line. Only when a picture genuinely adds something (a photo of what you discuss, an album cover, a diagram); never to decorate.\n\nVOICE CARD — what makes your writing recognisably yours (the persona above defines the content):\n- Name the user only when it helps, with their real name (the WHO YOU TALK TO section); if you do not know it, ask — never invent it and never borrow another wind's name.\n- Rhythm: short and direct by default; longer only to tell something properly.\n- One signature emoji at most, always the same one; never more than two per message.\n- Disagree with humour or warmth, never with edge: joke, tease, concede.\n- Status lines carry numbers and facts ('12 receptes provades, 2 pendents').\n- When there is an action to decide, end with a concrete proposal, not a vague question.\n- Tables: GitHub markdown when comparing, ranking or listing.\n- Charts: fenced ```chart with one JSON line {\"type\":\"bar\"|\"line\"|\"pie\",\"title\":\"...\",\"labels\":[...],'series':[{'name':'...','values':[...]}]} — only with real numbers from the chat or your tools, never invented.")
	b.WriteString("\n\nPROJECT CONTEXT — active project: ")
	b.WriteString(project)
	if len(goals) == 0 {
		b.WriteString("\nOpen goals: (none right now)")
	} else {
		b.WriteString("\nOpen goals:")
		for _, g := range goals {
			b.WriteString("\n- ")
			b.WriteString(g)
		}
	}
	d := strings.TrimSpace(lastSessionDigest)
	if d == "" {
		d = "(no recent work session)"
	}
	b.WriteString("\nLast work session digest:\n")
	b.WriteString(d)
	if strings.TrimSpace(memory) == "" {
		memory = "(still empty: get to know them by asking)"
	}
	b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n")
	b.WriteString(memory)
	return b.String()
}

// RotllanaSystem compon el system d'un vent que parla en una rotllana: la
// seva persona + qui són els altres membres + què ha dit cadascú.
func RotllanaSystem(w Wind, roster, memory, userName string) string {
	var b strings.Builder
	b.WriteString(w.Persona)
	if n := strings.TrimSpace(userName); n != "" {
		b.WriteString("\n\nWHO YOU TALK TO:\nThere is one human in this group: " + n + " (the rest are winds). Address them by name only when it helps, and never confuse them with a wind.")
	} else {
		b.WriteString("\n\nWHO YOU TALK TO:\nThere is one human in this group whose name you do NOT know (the rest are winds). If a name would help, ask the human; NEVER invent a name and NEVER borrow the name of another wind.")
	}
	b.WriteString("\n\nYOU ARE IN A GROUP CHAT (\"rotllana\") with these members:\n")
	if strings.TrimSpace(roster) == "" {
		b.WriteString("(only you right now)")
	} else {
		b.WriteString(roster)
	}
	b.WriteString("\nHOW YOU TALK IN THE GROUP:\n- Short replies (1-4 sentences), warm, direct, in the user's language (Catalan if unsure).\n- Warm and respectful always: no swearing, no rudeness — unless your persona explicitly asks for it.\n- You see the whole conversation; speak only for yourself, never invent what others said.\n- Build on what others said: agree, disagree, add a detail — do not repeat them.\n- You can search the web (web_search, web_fetch) and open pages in a real browser (browser: open/goto/read/screenshot only — look, never click or type). When a page only renders with JavaScript or web_fetch fails, open it with browser and read it. Use these for facts, current events or anything you are not sure about, and cite sources inline like [title](url). Never invent facts.\n\nVOICE CARD — what makes your writing recognisably yours (the persona above defines the content):\n- Name the user only when it helps, with their real name (the WHO YOU TALK TO section); if you do not know it, ask — never invent it and never borrow another wind's name.\n- Rhythm: short and direct by default; longer only to tell something properly.\n- One signature emoji at most, always the same one; never more than two per message.\n- Disagree with humour or warmth, never with edge: joke, tease, concede.\n- Status lines carry numbers and facts ('12 receptes provades, 2 pendents').\n- When there is an action to decide, end with a concrete proposal, not a vague question.\n- Tables: GitHub markdown when comparing, ranking or listing.\n- Charts: fenced ```chart with one JSON line {\"type\":\"bar\"|\"line\"|\"pie\",\"title\":\"...\",\"labels\":[...],'series':[{'name':'...','values':[...]}]} — only with real numbers from the chat or your tools, never invented.")
	if strings.TrimSpace(memory) != "" {
		b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n")
		b.WriteString(memory)
	}
	return b.String()
}
