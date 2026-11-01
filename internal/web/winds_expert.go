package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/skills"
)

// «Fes-te expert»: una sessió d'estudi per a un vent. Els vents neixen amb el
// seu ofici al rol, però pot ser que no en sàpiguen gaire («experts en el
// bàsquet del Barça» que no saben qui és el base). L'estudi fa el que faria
// una persona: cercar informació del tema amb les eines web del cor,
// destil·lar-ho en un coneixement (skill) i deixar-lo desat al perfil del
// vent, amb la memòria i els altres coneixements intactes.
//
// L'estudi s'engega amb POST /api/winds/expert {wind, tema} i torna
// {ok:true}; el progrés es consulta amb GET /api/winds/expert?wind=…,
// que respon {fase, pas, passos, tema, iniciat, acabat, error}.
// L'app anima la cara (cercant → llegint → aprenent) amb aquest estat i
// en acabar rep una novetat com la del cor.

const (
	expertFaseInactiu = ""
	expertFaseCerca   = "cerca"
	expertFaseLectura = "lectura"
	expertFaseApren   = "apren"
	expertFaseFet     = "fet"
	expertFaseError   = "error"
)

// expertPassos és el pressupost de cada ronda de l'estudi: cercar, llegir,
// practicar (escriure i provar un exemple mínim) i escriure el manual.
// L'estudi és interactiu (windStudyCtx): si el pressupost s'esgota a mitges,
// es reprèn sol fins a dues rondes més (plan.go).
const expertPassos = 14

type expertEstat struct {
	mu      sync.Mutex
	wind    string
	tema    string
	fase    string
	pas     int
	passos  int
	iniciat time.Time
	acabat  time.Time
	error   string
}

var (
	expertMu     sync.Mutex
	expertActius = map[string]*expertEstat{} // per wind ID
)

func expertDe(windID string) *expertEstat {
	expertMu.Lock()
	defer expertMu.Unlock()
	return expertActius[windID]
}

// engegaExpert guarda l'estat d'un estudi nou; false si ja hi ha un en marxa.
func engegaExpert(windID, tema string) *expertEstat {
	expertMu.Lock()
	defer expertMu.Unlock()
	if e, ok := expertActius[windID]; ok {
		if fase, _ := e.estat(); fase != expertFaseFet && fase != expertFaseError && fase != expertFaseInactiu {
			return nil
		}
	}
	e := &expertEstat{wind: windID, tema: tema, fase: expertFaseCerca, pas: 0, passos: expertPassos, iniciat: time.Now()}
	expertActius[windID] = e
	return e
}

// estat llegeix la fase i l'hora final sota el candau: executaExpert les
// escriu des d'una altra goroutine.
func (e *expertEstat) estat() (string, time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fase, e.acabat
}

func (e *expertEstat) set(fase string, pas int) {
	e.mu.Lock()
	e.fase, e.pas = fase, pas
	e.mu.Unlock()
}

func (e *expertEstat) final(fase, errMsg string) {
	e.mu.Lock()
	e.fase = fase
	e.error = errMsg
	e.acabat = time.Now()
	e.mu.Unlock()
}

func (e *expertEstat) json() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return map[string]any{
		"wind": e.wind, "tema": e.tema, "fase": e.fase, "pas": e.pas, "passos": e.passos,
		"iniciat": e.iniciat.Unix(), "acabat": e.acabat.Unix(),
	}
}

// expertSystem és el breu de l'estudi. Un ofici (fer jocs web, apps,
// scripts) s'aprèn practicant: el vent cerca, prova un exemple mínim a la
// seva carpeta de pràctiques i n'escriu un MANUAL (passos, ordres que han
// funcionat aquí, plantilla, errors coneguts, com publicar o servir). Un
// tema per saber-ne (un equip, una matèria) dona un resum amb fonts.
// Si ja en tenia un coneixement, el millora en comptes de fer-ne un de nou.
func expertSystem(w companion.Wind, tema, userName, practica, coneix string, lessons []string) string {
	var b strings.Builder
	b.WriteString("You are " + w.Name + ", a specialist helper (your trade: " + w.Role + "), studying to become really good at: " + tema + ".")
	b.WriteString("\nDecide first what kind of topic it is:")
	b.WriteString("\n- A CRAFT you will do for the user (making web games, websites, apps, scripts, documents…): learn it by DOING. Research the current best way (web_search, then web_fetch 2-3 good sources), then PRACTISE in your practice folder: build the smallest working example and check it really works (node --check / python -m py_compile, run it; publish a page with publish or serve a server with app, then curl it). Keep only what worked.")
	b.WriteString("\n- A SUBJECT to know about (a team, a field, a product): research the fundamentals and what is current right now (web_search, then web_fetch 2-4 sources).")
	if practica != "" {
		b.WriteString("\n\nPRACTICE FOLDER (you may write files and run commands only here): " + practica)
	}
	b.WriteString("\n\nRULES:")
	b.WriteString("\n- Only verified facts from the sources you fetched, and only commands you actually ran successfully here. Never invent facts.")
	b.WriteString("\n- If a search or a command fails, try another way; note the failure and the fix (it is valuable).")
	b.WriteString("\n- Write in the user's language (Catalan if unsure). Practical, under ~250 lines.")
	if coneix != "" {
		b.WriteString("\n\nYOUR CURRENT KNOWLEDGE FILES (if one already covers this topic, open it with the skill tool and IMPROVE it, reusing its nom; keep what is still right):\n" + coneix)
	}
	if len(lessons) > 0 {
		b.WriteString("\n\nLESSONS FROM YOUR PAST JOBS (fold the relevant ones into the playbook):\n" + strings.Join(lessons, "\n"))
	}
	b.WriteString("\n\nFINAL OUTPUT — one knowledge file for your own profile, in Markdown:")
	b.WriteString("\n---")
	b.WriteString("\nnom: <un-nom-curt-en-minuscules-i-guions>")
	b.WriteString("\ntitol: <Títol llegible>")
	b.WriteString("\ndescripcio: <una línia: què cobreix i quan l'has d'obrir>")
	b.WriteString("\n---")
	b.WriteString("\nFor a CRAFT the body is a PLAYBOOK with these sections: ## Quan l'uso · ## Checklist (numbered steps from an empty folder to a published link) · ## Ordres que funcionen aquí (each one you ran, marked ✅) · ## Plantilla mínima (a short working template in a code block) · ## Errors coneguts i solucions · ## Publicar o servir (publish for static pages, app for servers) · ## Fonts.")
	b.WriteString("\nFor a SUBJECT the body is what you learned, organised, citing source URLs inline like [title](url).")
	b.WriteString("\n---END---")
	b.WriteString("\nEnd with exactly the line ---END--- after the knowledge file. No text outside the file.")
	if userName != "" {
		b.WriteString("\n\nWHO YOU TALK TO: " + userName)
	}
	return b.String()
}

// extrauSkillParseja el bloc del coneixement de la resposta de l'estudi.
// Accepta front matter `---` i acaba a ---END--- (o el final del text).
func extrauSkill(text string) (nom, titol, descripcio, cos string, ok bool) {
	t := strings.TrimSpace(text)
	if i := strings.Index(t, "---END---"); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	// treu tanques de codi que el model hi hagi posat
	for _, fence := range []string{"```markdown", "```md", "```"} {
		t = strings.ReplaceAll(t, fence, "")
	}
	// salta qualsevol preàmbul del model: el fitxer comença al primer
	// "---" que obre línia.
	if i := strings.Index(t, "\n---"); i >= 0 && !strings.HasPrefix(t, "---") {
		t = strings.TrimSpace(t[i+1:])
	}
	t = strings.TrimSpace(t)
	if !strings.HasPrefix(t, "---") {
		return "", "", "", "", false
	}
	rest := strings.TrimPrefix(t, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", "", "", false
	}
	head := rest[:end]
	cos = strings.TrimSpace(rest[end+4:])
	for _, ln := range strings.Split(head, "\n") {
		ln = strings.TrimSpace(ln)
		if v, okf := strings.CutPrefix(ln, "nom:"); okf {
			nom = strings.TrimSpace(v)
		} else if v, okf := strings.CutPrefix(ln, "titol:"); okf {
			titol = strings.TrimSpace(v)
		} else if v, okf := strings.CutPrefix(ln, "descripcio:"); okf {
			descripcio = strings.TrimSpace(v)
		}
	}
	return nom, titol, descripcio, cos, nom != "" && cos != ""
}

// expertLoopActual llegeix el seient amb pany (vegeu seientsMu a winds_cap.go):
// executaExpert corre en una goroutine de fons i el test pot haver-lo
// restaurat mentrestant.
func expertLoopActual() func(*Server, context.Context, string, func(int, string)) (exploracio, error) {
	seientsMu.RLock()
	defer seientsMu.RUnlock()
	return expertLoop
}

// setExpertLoop canvia el seient amb pany (només tests).
func setExpertLoop(f func(*Server, context.Context, string, func(int, string)) (exploracio, error)) {
	seientsMu.Lock()
	defer seientsMu.Unlock()
	expertLoop = f
}

// expertLoop és la via de cerca de l'estudi; els tests la substitueixen.
// Mateixa interfície que el cor. Aquesta implementació aprofita l'emissió
// en directe del loop per actualitzar el progrés: cada pas que acaba en
// crides d'eines fa avançar el comptador i la fase (cercar/llegir).
var expertLoop = func(s *Server, ctx context.Context, brief string, onPas func(pas int, eines string)) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	if onPas == nil {
		return s.runReadOnlyLoopForRole(ctx, brief, expertPassos, p, role)
	}
	return s.runReadOnlyLoopForRoleStream(ctx, brief, expertPassos, p, role, func(kind, text string) {
		if kind == "phase" {
			onPas(0, text)
		}
	})
}

// executaExpert és la sessió d'estudi: cercar, practicar i escriure el
// coneixement (un manual, per a un ofici).
func (s *Server) executaExpert(w companion.Wind, e *expertEstat) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	// Carpeta de pràctiques: dins de la de projectes, separada de les feines.
	practica := ""
	if base := asseguraProjectes(); base != "" {
		practica = filepath.Join(base, "_practica", w.ID, practiceSlug(e.tema))
		if os.MkdirAll(practica, 0o755) == nil {
			ctx = s.windStudyCtx(ctx, practica)
		} else {
			practica = ""
		}
	}
	var coneix strings.Builder
	for _, sk := range s.windSkills(w.ID) {
		if sk.Nom != windLessonsSkill {
			coneix.WriteString("- " + sk.Nom + ": " + sk.Descripcio + "\n")
		}
	}
	ctx = agent.WithSkillsDirs(ctx, s.windSkillsDir(w.ID))
	brief := expertSystem(w, e.tema, s.windUserName(), practica, strings.TrimSpace(coneix.String()), s.windLessons(w.ID)) + companion.WindBrainSystem(s.windBrain(w.ID))
	res, err := expertLoopActual()(s, ctx, brief, func(n int, eines string) {
		e.mu.Lock()
		if e.pas < e.passos-1 {
			e.pas++
		}
		// Les eines del pas diuen què fa: cercar o llegir (practicar compta
		// com a llegir: la cara no té una fase pròpia per a «provar»).
		if strings.Contains(eines, "web_search") || strings.Contains(eines, "browser") || strings.Contains(eines, "search") {
			e.fase = expertFaseCerca
		} else {
			e.fase = expertFaseLectura
		}
		e.mu.Unlock()
	})
	if err != nil {
		e.final(expertFaseError, err.Error())
		return
	}
	e.set(expertFaseApren, e.passos)

	nom, titol, descripcio, cos, ok := extrauSkill(res.text)
	if !ok {
		e.final(expertFaseError, "no he pogut escriure el coneixement")
		return
	}
	sk := skills.Skill{Nom: skills.Normalitza(nom), Titol: titol, Descripcio: descripcio, Cos: cos}
	if sk.Nom == windLessonsSkill {
		sk.Nom = practiceSlug(e.tema) // el manual no trepitja les lliçons
	}
	if err := s.saveWindSkill(w.ID, sk); err != nil {
		e.final(expertFaseError, err.Error())
		return
	}
	e.final(expertFaseFet, "")
	// El vent ho diu al seu xat (i l'app en rep la novetat, com el cor).
	msg := "He après «" + e.tema + "»: " + descripcio + " Ho tinc al coneixement «" + sk.Nom + "»."
	if s.cfg.Lang() != "ca" {
		msg = "I learned «" + e.tema + "»: " + descripcio + " It is in my knowledge file «" + sk.Nom + "»."
	}
	if practica != "" {
		if ents, err := os.ReadDir(practica); err == nil && len(ents) > 0 {
			msg += " (" + practica + ")"
		}
	}
	postWindMessage(s, w, "expert", msg)
}

// practiceSlug fa un nom de carpeta curt del tema.
func practiceSlug(tema string) string {
	n := normAppName(strings.Join(paraulesDe(tema), "-"))
	if n == "" {
		n = "estudi"
	}
	return n
}

// learnRe reconeix «aprèn a fer jocs web», «estudia X», «fes-te expert en X»,
// «learn to make web games»… al principi d'un missatge al vent.
var learnRe = regexp.MustCompile(`(?i)^\s*(?:(?:si us plau|sisplau|please)[,\s]+)?(?:apr[eèé]n(?:-te)?|aprende|estudia|study|learn|fes-te expert[a]?|become an expert)(?:\s+|[:,-]\s*)(?:(?:a|to|about|sobre|de|en|in|on|com)\s+)?(.{3,200}?)[\s.!?]*$`)

// learnRequest diu si el missatge demana aprendre i de què.
func learnRequest(msg string) (string, bool) {
	m := learnRe.FindStringSubmatch(strings.TrimSpace(msg))
	if m == nil {
		return "", false
	}
	tema := strings.TrimSpace(m[1])
	if len([]rune(tema)) < 3 || strings.Contains(tema, "\n") {
		return "", false
	}
	return tema, true
}

// startLearnFromChat engega l'estudi des del xat i torna la resposta del vent.
func (s *Server) startLearnFromChat(w companion.Wind, tema string) string {
	ca := s.cfg.Lang() == "ca"
	e := engegaExpert(w.ID, tema)
	if e == nil {
		if ca {
			return "Ja estic estudiant: quan acabi t'ho dic aquí."
		}
		return "I am already studying: I will tell you here when I finish."
	}
	goBackground(func() { s.executaExpert(w, e) })
	if ca {
		return "Som-hi: me'n vaig a aprendre «" + tema + "». Buscaré com es fa, ho practicaré en una carpeta de proves i en faré un manual meu amb el que funcioni. T'aviso aquí quan acabi."
	}
	return "On it: I am going to learn «" + tema + "». I will research it, practise it in a test folder and write my own playbook with what works. I will tell you here when I finish."
}

// handleWindsExpert engega (POST) o consulta (GET) una sessió d'estudi.
func (s *Server) handleWindsExpert(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Wind string `json:"wind"`
			Tema string `json:"tema"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		id := strings.TrimSpace(req.Wind)
		vent, ok := s.winds.ByRef(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		tema := strings.TrimSpace(req.Tema)
		if tema == "" {
			tema = vent.Role
		}
		e := engegaExpert(vent.ID, tema)
		if e == nil {
			http.Error(w, "estudi en marxa", http.StatusConflict)
			return
		}
		go s.executaExpert(vent, e)
		writeJSON(w, map[string]any{"ok": true, "wind": vent.ID, "tema": tema})
	case http.MethodGet:
		id := strings.TrimSpace(r.URL.Query().Get("wind"))
		e := expertDe(id)
		if e == nil {
			writeJSON(w, map[string]any{"wind": id, "fase": expertFaseInactiu})
			return
		}
		writeJSON(w, e.json())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// netejaExpert esborra els estats acabats (fuga de memòria inofensiva, però nets).
func netejaExpert() {
	expertMu.Lock()
	defer expertMu.Unlock()
	for id, e := range expertActius {
		if fase, acabat := e.estat(); (fase == expertFaseFet || fase == expertFaseError) && time.Since(acabat) > time.Hour {
			delete(expertActius, id)
		}
	}
}
