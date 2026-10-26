package web

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

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

const expertPassos = 8 // mateixa amplada que el cor: cerca àmplia, no superficial

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

// expertSystem és el breu de l'estudi: cerca com el cor, però el producte
// final és un coneixement, no una notícia.
func expertSystem(w companion.Wind, tema, userName string) string {
	var b strings.Builder
	b.WriteString("You are " + w.Name + ", studying to become a real expert on: " + tema + ".")
	b.WriteString("\nYour job is to research this topic on the web and write down what matters.")
	b.WriteString("\n\nRULES:")
	b.WriteString("\n- Search the web for the fundamentals and the current state of the topic: web_search first, then web_fetch the most promising 2-4 sources.")
	b.WriteString("\n- Cover: key facts, main entities (people, teams, projects), how it works today, and what is current right now.")
	b.WriteString("\n- Only verified facts from the sources you fetch. Cite source URLs inline like [title](url).")
	b.WriteString("\n- Never invent facts. If a search fails, try a different query; if all fails, say so.")
	b.WriteString("\n- Write in the user's language (Catalan if unsure).")
	b.WriteString("\n\nFINAL OUTPUT — a knowledge file for your own profile, in Markdown:")
	b.WriteString("\n---")
	b.WriteString("\nnom: <un-nom-curt-en-minuscules-i-guions>")
	b.WriteString("\ntitol: <Títol llegible>")
	b.WriteString("\ndescripcio: <una línia que digui què cobreix>")
	b.WriteString("\n---")
	b.WriteString("<Cos en Markdown: el que has après, organitzat, amb les fonts citades>")
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

// executaExpert és la sessió d'estudi: cerca + escriure coneixement.
func (s *Server) executaExpert(w companion.Wind, e *expertEstat) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	brief := expertSystem(w, e.tema, s.windUserName()) + companion.WindBrainSystem(s.windBrain(w.ID))
	res, err := expertLoop(s, ctx, brief, func(n int, eines string) {
		e.mu.Lock()
		e.pas++
		// Les eines del pas diuen què fa: cercar o llegir.
		if strings.Contains(eines, "web_search") || strings.Contains(eines, "browser") || strings.Contains(eines, "search") {
			e.fase = expertFaseCerca
		} else if strings.Contains(eines, "web_fetch") || strings.Contains(eines, "read") {
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
	if err := s.saveWindSkill(w.ID, sk); err != nil {
		e.final(expertFaseError, err.Error())
		return
	}
	e.final(expertFaseFet, "")
	// Com el cor: una novetat perquè l'app sàpiga que ja és expert.
	recordNovetat(s, w, "expert", w.Name+" s'ha fet expert en "+e.tema)
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
