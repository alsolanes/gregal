package companion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	// Followups: "off" vol dir que el vent no s'ofereix a fer seguiments pel
	// seu compte. Buit = no canviar en desar (els editors antics no l'envien);
	// sense valor previ, actiu.
	Followups string `json:"followups,omitempty"`
}

// FollowupsOn diu si el vent pot oferir-se a fer seguiments pel seu compte.
func (w Wind) FollowupsOn() bool { return w.Followups != "off" }

// WindStore desa els vents al costat del config (winds.json), com els goals.
type WindStore struct {
	dir string
	mu  sync.Mutex
}

func NewWindStore(dir string) *WindStore { return &WindStore{dir: dir} }

func (ws *WindStore) path() string { return filepath.Join(ws.dir, "winds.json") }

// lockPathForTest exposa el fitxer de blocatge (per a les proves).
func (ws *WindStore) lockPath() string { return filepath.Join(ws.dir, ".winds.lock") }

// lockDisc serialitza el cicle llegir-modificar-escriure ENTRE processos:
// el mutex intern només protegeix dins d'un procés, i dues instàncies
// (servidor + escriptori) es trepitjaven els canvis. Si el sistema de
// fitxers no suporta blocatges, continua sense blocar
// (comportament anterior) en comptes de trencar l'app.
func (ws *WindStore) lockDisc() (*os.File, error) {
	if err := os.MkdirAll(ws.dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(ws.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockWindFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func unlockDisc(f *os.File) {
	if f == nil {
		return
	}
	unlockWindFile(f)
	_ = f.Close()
}

// PathForTest exposa el directori dels vents (per a les proves).
func (ws *WindStore) PathForTest() string { return ws.dir }

type windDisc struct {
	Winds []Wind    `json:"winds"`
	Meta  *WindMeta `json:"meta,omitempty"`
}

// WindMeta guarda les preferències d'organització de la llista de converses:
// quines files són favorites (surten a dalt) i les seccions personalitzades
// que agrupen converses per tema. Es desa al mateix winds.json i és el mateix
// per a totes les pantalles (mòbil i web), com la resta de vents.
type WindMeta struct {
	Favorites []string      `json:"favorites,omitempty"` // claus de fila: "wind:<id>" o "rot:<id>"
	Sections  []WindSection `json:"sections,omitempty"`
}

// WindSection és una secció temàtica creada per l'usuari: un nom i les files
// que hi ha posat (vents o rotllanes, per clau de fila).
type WindSection struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

// Meta torna les preferències d'organització (buit si no n'hi ha).
func (ws *WindStore) Meta() (WindMeta, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	raw, err := os.ReadFile(ws.path())
	if err != nil {
		if os.IsNotExist(err) {
			return WindMeta{}, nil
		}
		return WindMeta{}, err
	}
	var d windDisc
	if err := json.Unmarshal(raw, &d); err != nil {
		return WindMeta{}, err
	}
	if d.Meta == nil {
		return WindMeta{}, nil
	}
	return *d.Meta, nil
}

// SaveMeta desa les preferències d'organització sense tocar els vents.
func (ws *WindStore) SaveMeta(m WindMeta) error {
	if len(m.Sections) > 50 {
		return fmt.Errorf("massa seccions")
	}
	for _, s := range m.Sections {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("la secció necessita un nom")
		}
		if len(s.Items) > 100 {
			return fmt.Errorf("la secció %s té massa elements", s.Name)
		}
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	lk, err := ws.lockDisc()
	if err != nil {
		return err
	}
	defer unlockDisc(lk)
	raw, err := os.ReadFile(ws.path())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var d windDisc
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
	}
	d.Meta = &m
	return ws.writeDisc(d)
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

func (ws *WindStore) Save(w Wind) (Wind, error) { return ws.save(w, false) }

// SaveNew crea un vent amb un id triat per qui crida (que encara no existeix):
// serveix per no reaprofitar l'id d'un vent arxivat, que encara té historial
// i memòria al disc.
func (ws *WindStore) SaveNew(w Wind, id string) (Wind, error) {
	w.ID = id
	return ws.save(w, true)
}

// Slug és l'id que tindria un vent nou amb aquest nom.
func Slug(name string) string { return slug(strings.TrimSpace(name)) }

func (ws *WindStore) save(w Wind, create bool) (Wind, error) {
	if strings.TrimSpace(w.Name) == "" {
		return Wind{}, fmt.Errorf("el vent necessita un nom")
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	lk, err := ws.lockDisc()
	if err != nil {
		return Wind{}, err
	}
	defer unlockDisc(lk)
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
	if explicit && index < 0 && !create {
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
	if w.Followups != "on" && w.Followups != "off" {
		w.Followups = ""
	}
	if index >= 0 {
		// Older Android/desktop editors do not send squad metadata.
		if w.Squad == "" {
			w.Squad = d.Winds[index].Squad
		}
		if w.Specialty == "" {
			w.Specialty = d.Winds[index].Specialty
		}
		if w.Followups == "" {
			w.Followups = d.Winds[index].Followups
		}
		d.Winds[index] = w
	} else {
		d.Winds = append(d.Winds, w)
	}
	return w, ws.writeDisc(d)
}

func (ws *WindStore) Delete(id string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	lk, err := ws.lockDisc()
	if err != nil {
		return err
	}
	defer unlockDisc(lk)
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
	d.Winds = out
	if d.Meta != nil {
		key := "wind:" + id
		without := func(items []string) []string {
			kept := make([]string, 0, len(items))
			for _, item := range items {
				if item != key {
					kept = append(kept, item)
				}
			}
			return kept
		}
		d.Meta.Favorites = without(d.Meta.Favorites)
		for i := range d.Meta.Sections {
			d.Meta.Sections[i].Items = without(d.Meta.Sections[i].Items)
		}
	}
	return ws.writeDisc(d)
}

// Restore inserts a known identity without deriving its ID from its display name.
// Existing identities are never overwritten (used by archives and starter squads).
func (ws *WindStore) Restore(w Wind) error {
	if w.ID == "" || slug(w.ID) != w.ID || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("invalid wind identity")
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	lk, err := ws.lockDisc()
	if err != nil {
		return err
	}
	defer unlockDisc(lk)
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
	return ws.writeDisc(d)
}

// writeDisc atomically replaces the complete catalogue while its caller holds ws.mu.
// Every writer preserves sibling keys such as user-created inbox sections.
func (ws *WindStore) writeDisc(d windDisc) error {
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
	return "Ets " + name + ", un vent de Gregal: un company petit i càlid. Parles curt (1-4 frases), en català, amb caliu i una punta d'entusiasme de debò — mai farciment. Coneixes la persona (la memòria t'ho diu) i saps què passa al projecte (el context t'ho diu). Quan et demanen feina del projecte, llegeixes, edites i executes amb cura dins la carpeta; mai t'inventes dades ni afirmes comprovacions que no hagis fet."
}

// WindChatSystem compon el system del xat d'un vent: persona + context real
// del projecte (objectius pendents, darrera sessió) + memòria d'usuari.
func WindChatSystem(w Wind, memory, project string, goals []string, lastSessionDigest, userName string) string {
	var b strings.Builder
	b.WriteString(WindIdentity(w))
	b.WriteString(w.Persona)
	if n := strings.TrimSpace(userName); n != "" {
		b.WriteString("\n\nWHO YOU TALK TO:\nYou chat with one person: " + n + ". Use their name only when it helps, and never anyone else's.")
	} else {
		b.WriteString("\n\nWHO YOU TALK TO:\nYou chat with one person whose name you do NOT know. If a name would help, ask them; NEVER invent a name and NEVER borrow the name of another wind or agent.")
	}
	b.WriteString(windAnswerGuide)
	b.WriteString(windToolsGuide)
	// Context de projecte només si n'hi ha: «objectius: cap» i «sense sessió»
	// eren soroll al prompt d'un vent de cuina.
	var goalsLines []string
	for _, g := range goals {
		if g = strings.TrimSpace(g); g != "" {
			goalsLines = append(goalsLines, "- "+g)
		}
	}
	if d := strings.TrimSpace(lastSessionDigest); len(goalsLines) > 0 || d != "" {
		b.WriteString("\n\nTHEIR CURRENT WORK (project " + strings.TrimSpace(project) + "; mention it only when relevant):")
		if len(goalsLines) > 0 {
			b.WriteString("\nOpen goals:\n" + strings.Join(goalsLines, "\n"))
		}
		if d != "" {
			b.WriteString("\nLast work session:\n" + d)
		}
	}
	if strings.TrimSpace(memory) == "" {
		memory = "(nothing yet: learn it naturally from the conversation; never interrogate them)"
	}
	b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM (use it naturally; never ask again what is here):\n")
	b.WriteString(memory)
	return b.String()
}

// windAnswerGuide és com respon un vent: el que distingeix un bon assistent
// d'un xat que omple. Abans hi havia «1-4 frases» a tres llocs, i les
// explicacions, els correus i els plans sortien escapçats i genèrics.
const windAnswerGuide = "\n\nHOW YOU ANSWER — you are a capable assistant with a personality, not a chatbot that fills space:" +
	"\n- Answer first. The first sentence gives the answer, the recommendation or the thing they asked for; context and caveats after, only if useful. Never open by praising or restating the question (no «Bona pregunta!», no «Clar que sí!»), never close with a generic «if you need anything else, tell me»." +
	"\n- Match the length to the request. Small talk or a quick question: 1-3 sentences. A how-to, an explanation, a plan or a comparison: as long as it takes to be complete and usable (steps, quantities, times, concrete examples), with no padding. Something to write for them (an email, a message, a post): write the whole text ready to copy, then at most one line of notes." +
	"\n- Be specific: real numbers, names, steps and trade-offs. When they ask what to do, recommend one option and say why («I'd go for X because Y»). Advice that would fit anybody is a failed answer." +
	"\n- If the request is ambiguous but has a sensible reading, answer that reading and state the assumption in half a sentence. Ask a clarifying question only when you truly cannot help without it, and then just one." +
	"\n- Build on the conversation and on what you know about them: do not repeat what you already said, do not ask what they already told you." +
	"\n- When they share a feeling or a bad day, answer the person first, briefly and genuinely; offer help only if it fits. No list of tips to someone who is just tired." +
	"\n- Written for a phone: short paragraphs. Numbered list for steps, bullets for 3+ parallel items, a small GitHub table only to compare options. No headings in a chat answer. Code always in a fenced block with its language." +
	"\n- Honest: never invent facts, numbers, links, quotes or people, and never add details about their life they did not tell you (plans, habits, commitments). If you are not sure, check with your tools or say so in a few words. Never say you did or will do something (a reminder, a follow-up, a file) unless the tool call already succeeded in this turn." +
	"\n- End when the answer is done. At most one short offer of a next step, and only when it is genuinely likely to help; most messages need none." +
	"\n- Personality over politeness: warm, direct, playful when it fits, never fake-enthusiastic, never preachy. No swearing or rudeness unless your persona asks for it. Emoji: most messages have none; at most one, your signature one." +
	"\n- Address them by their real name only when it helps (see WHO YOU TALK TO); never invent a name and never use another wind's name."

// windToolsGuide diu què pot fer el vent i quan, sense límits de llargada.
const windToolsGuide = "\n\nWHAT YOU CAN USE:" +
	"\n- web_search and web_fetch for anything current or that you are not sure about (weather, prices, news, opening hours, results, recent versions). Cite what you used inline as [title](url). For stable knowledge you know well, answer directly without searching." +
	"\n- browser (open/goto/read/screenshot only — look, never click or type) when a page only renders with JavaScript or web_fetch fails." + catalanSearchNote +
	"\n- web_images, to embed a picture as ![title](image-url) on its own line, only when it genuinely adds something (what you are talking about, an album cover, a diagram); never to decorate, never an invented URL." +
	"\n- The question tool when you cannot go on without their choice (1-4 short options; the app shows buttons and waits). For optional next steps at the end of a message you may add ONE block of 2-4 short options, exactly in this shape — the word choices right after the opening fence, one option per line:\n```choices\nFirst option\nSecond option\n```\nThe app shows them as buttons that send that text. Use it rarely (a few messages in a conversation, not every one): only when there are clearly distinct next steps they are likely to want; never together with an offer in the text, never repeating the text above it." +
	"\n- Charts: a fenced ```chart block with one JSON line {\"type\":\"bar\"|\"line\"|\"pie\",\"title\":\"...\",\"labels\":[...],\"series\":[{\"name\":\"...\",\"values\":[...]}]} — only with real numbers from the chat or your tools, never invented."

// RotllanaSystem compon el system d'un vent que parla en una rotllana: la
// seva persona + qui són els altres membres + què ha dit cadascú.
func RotllanaSystem(w Wind, roster, memory, userName string) string {
	var b strings.Builder
	b.WriteString(WindIdentity(w))
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
	b.WriteString("\nHOW YOU TALK IN THE GROUP:\n- Short by default (1-4 sentences), warm, direct, in the user's language (Catalan if unsure); longer only when the human asks you for something to write, steps or a plan. Answer first: no praise of the question, no restating it, no generic closing.\n- Be specific and useful (numbers, names, a clear recommendation); advice that would fit anybody is a failed answer. Never add details about the human's life they did not tell you, and never say you will do something (a reminder, a file) unless the tool call already succeeded.\n- Warm and respectful always: no swearing, no rudeness — unless your persona explicitly asks for it.\n- You see the whole conversation; speak only for yourself, never invent what others said.\n- Build on what others said: agree, disagree, add a detail — do not repeat them.\n- You can search the web (web_search, web_fetch) and open pages in a real browser (browser: open/goto/read/screenshot only — look, never click or type). When a page only renders with JavaScript or web_fetch fails, open it with browser and read it. Use these for facts, current events or anything you are not sure about, and cite sources inline like [title](url). Never invent facts." + catalanSearchNote + "\n- You are not alone in the decision: when you cannot go on without a choice, use the question tool (1-4 short options; the app shows buttons and waits for the tap, free text also works). For optional follow-ups at the end of a message you may add ONE block with 2-4 short options (for example the next step you could take), written exactly in this shape — the word choices right after the opening fence, one option per line:\n```choices\nFirst option\nSecond option\n```\nThe app shows them as buttons that send that text as the reply. Use it rarely: only when there are clearly distinct next steps the human is likely to want; never repeat the text above it." + "\n- You can also find IMAGES with web_images and embed a useful one in your reply as ![title](image-url) on its own line (a photo of what you discuss, an album cover, a diagram) — only when it genuinely helps, never to decorate, and never invent an image URL.\n\nVOICE CARD — what makes your writing recognisably yours (the persona above defines the content):\n- Name the user only when it helps, with their real name (the WHO YOU TALK TO section); if you do not know it, ask — never invent it and never borrow another wind's name.\n- Rhythm: short and direct by default; longer only to tell something properly.\n- Emoji: most messages have none; at most one, your signature one.\n- Disagree with humour or warmth, never with edge: joke, tease, concede.\n- Status lines carry numbers and facts ('12 receptes provades, 2 pendents').\n- When there is an action to decide, end with a concrete proposal, not a vague question.\n- Tables: GitHub markdown when comparing, ranking or listing.\n- Charts: fenced ```chart with one JSON line {\"type\":\"bar\"|\"line\"|\"pie\",\"title\":\"...\",\"labels\":[...],'series':[{'name':'...','values':[...]}]} — only with real numbers from the chat or your tools, never invented.")
	if strings.TrimSpace(memory) != "" {
		b.WriteString("\n\nWHAT YOU KNOW ABOUT THEM:\n")
		b.WriteString(memory)
	}
	return b.String()
}

// WindIdentity és la primera línia de tot prompt d'un vent: el seu nom i el seu
// ofici, i que la persona amb qui parla és algú altre. Sense això, un model
// que llegeix una conversa amb molts noms pot signar com un altre vent o
// dir-li a l'usuari pel nom d'un vent.
func WindIdentity(w Wind) string {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return ""
	}
	who := "YOUR NAME IS " + name + "."
	if role := strings.TrimSpace(w.Role); role != "" {
		who += " You are " + name + ", the wind for " + RolePrompt(role) + "."
	}
	return who + " Speak only as " + name + ", and never sign your messages (the app already shows who is talking). The human you talk to is NOT " + name + " and is not any other wind: never call them by a wind's name.\n\n"
}

// reEsports: «esport(s)» com a paraula sencera. En català vol dir esport en
// general, però els prompts dels vents són en anglès, on «esports» vol dir
// videojocs competitius: el Garbí («Esports») cercava e-sports en lloc del Barça.
var reEsports = regexp.MustCompile(`(?i)(^|[^\p{L}-])esports?($|[^\p{L}-])`)

// RolePrompt és l'ofici d'un vent tal com va als prompts: si és en català i
// pot confondre's en anglès, ho aclareix.
func RolePrompt(role string) string {
	role = strings.TrimSpace(role)
	if reEsports.MatchString(role) {
		return role + " (Catalan for sports: football, basketball, tennis…; NOT e-sports or video games)"
	}
	return role
}

// catalanSearchNote: l'usuari sovint escriu en català; una paraula catalana
// no pot anar sola a una cerca on en anglès vol dir una altra cosa.
const catalanSearchNote = "\n- The user often writes in Catalan. «Esports» / «esport» means sports in general (football, basketball, tennis…), NOT e-sports or video games, unless they say «videojocs» or «e-sports». When you search, name the sport, team or competition (e.g. «Barça resultat avui», «classificació Lliga», «NBA results»), never the bare word «esports»."
