package server

// Els equips de la lliga (Partits) viuen al perfil de Gregal Jocs, no als
// vents de Gregal: la lliga no llegeix cap vent ni escriu cap fitxer de
// Gregal («Avui», veus dels vents).
//
// Model de dades (tot JSON, a <estat de l'usuari>/football/):
//
//	members.json  {"version":1,"members":[Member…]}
//	              qui hi ha a la lliga: els entrenadors, les estrelles i el
//	              comentarista. Si no n'hi ha, se'n crea el joc per defecte
//	              (defaultMembers) la primera vegada que es demana.
//	league.json   football.League: les dues franquícies (23 jugadors cadascuna,
//	              els membres hi són les estrelles i els entrenadors) i el
//	              rècord. Es fa amb football.NewLeague a partir dels membres.
//	games/<id>.json  els partits (football.go).
//
// Tot passa per TeamStore perquè aviat els equips seran «equips d'agents
// amb instruccions per posició»: n'hi haurà prou de canviar la
// implementació (o el que guarda Member) sense tocar el motor ni l'API.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gregal/jocs/football"
)

// Member és un membre de la lliga: un personatge amb nom, cara i veu.
// Els dos primers que reparteix la lliga en són els entrenadors, i un
// tercer, si n'hi ha, el comentarista.
type Member struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"`      // a què es dedica (tria la posició)
	Specialty string `json:"specialty,omitempty"` // també ajuda a triar la posició
	Color     string `json:"color,omitempty"`     // #RRGGBB
	Style     string `json:"style,omitempty"`     // cara (ulls, boca, forma)
	Persona   string `json:"persona,omitempty"`   // com parla als prompts
	// Staff: les instruccions de cada posició de l'equip que entrena (id de
	// posició → text). Les llegeix el pla de cada quart (fbCoachPlan).
	Staff map[string]string `json:"staff,omitempty"`
}

// staffPosition és una posició del cos tècnic i els comandaments del pla que decideix.
type staffPosition struct{ ID, Name, Dials, Starter string }

// nflStaff: el cos tècnic d'un equip de la lliga. Són les mateixes posicions
// de NFL que l'Arena (arena-positions.js); a la lliga decideixen el pla del quart.
var nflStaff = []staffPosition{
	{"head_coach", "Head coach", "aggression, tempo", "Go for it on 4th & short past midfield. Speed up when trailing late."},
	{"oc", "Offensive coordinator", "pass_rate, deep_shots", "Balanced: pass a bit more on early downs, take deep shots when the safeties creep up."},
	{"qb", "Quarterback", "focus", "Feature the best receiver; go to the tight end in the red zone."},
	{"dc", "Defensive coordinator", "blitz", "Blitz on obvious passing downs, stay in coverage otherwise."},
}

// staffBrief és el que llegeix el model: qui decideix cada comandament.
func staffBrief(staff map[string]string) string {
	var b strings.Builder
	b.WriteString("YOUR STAFF: each agent decides its own dials of the plan, following its instructions.\n")
	for _, p := range nflStaff {
		ins := strings.TrimSpace(staff[p.ID])
		if ins == "" {
			ins = p.Starter
		}
		b.WriteString("- " + p.Name + " (decides " + p.Dials + "): " + ins + "\n")
	}
	return b.String()
}

// TeamStore guarda els membres i la lliga d'un usuari.
type TeamStore interface {
	// Members torna els membres de la lliga; si encara no n'hi ha cap, en
	// crea el joc per defecte i el desa.
	Members() ([]Member, error)
	// League torna la lliga desada (nil si encara no n'hi ha).
	League() (*football.League, error)
	SaveLeague(*football.League) error
	// ResetLeague esborra la lliga (no els membres): la propera es refà.
	ResetLeague() error
	// SaveMembers desa els membres (el cos tècnic de cada equip, p. ex.).
	SaveMembers([]Member) error
}

// fileTeamStore és el TeamStore de fitxers JSON en una carpeta.
type fileTeamStore struct{ dir string }

func newFileTeamStore(dir string) *fileTeamStore { return &fileTeamStore{dir: dir} }

type membersFile struct {
	Version int      `json:"version"`
	Members []Member `json:"members"`
}

func (st *fileTeamStore) Members() ([]Member, error) {
	path := filepath.Join(st.dir, "members.json")
	raw, err := os.ReadFile(path)
	if err == nil {
		var f membersFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		if len(f.Members) > 0 {
			return f.Members, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ms := defaultMembers()
	if err := writeJSONFile(path, membersFile{Version: 1, Members: ms}); err != nil {
		return nil, err
	}
	return ms, nil
}

func (st *fileTeamStore) SaveMembers(ms []Member) error {
	return writeJSONFile(filepath.Join(st.dir, "members.json"), membersFile{Version: 1, Members: ms})
}

func (st *fileTeamStore) League() (*football.League, error) {
	raw, err := os.ReadFile(filepath.Join(st.dir, "league.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lg football.League
	if err := json.Unmarshal(raw, &lg); err != nil {
		return nil, nil // una lliga malmesa es refà
	}
	return &lg, nil
}

func (st *fileTeamStore) SaveLeague(lg *football.League) error {
	return writeJSONFile(filepath.Join(st.dir, "league.json"), lg)
}

func (st *fileTeamStore) ResetLeague() error {
	err := os.Remove(filepath.Join(st.dir, "league.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// writeJSONFile desa v de manera atòmica (temporal + rename), 0600.
func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// defaultMembers és el joc de membres d'una lliga nova: dos entrenadors i
// un comentarista, vents d'aquí (els jocs parlen anglès).
func defaultMembers() []Member {
	return []Member{
		{ID: "tramuntana", Name: "Tramuntana", Role: "strategy lead", Specialty: "planning", Color: "#3E7BFA", Style: "dso",
			Persona: "You are blunt, cold-headed and love a disciplined plan. Short sentences, no fluff."},
		{ID: "garbi", Name: "Garbí", Role: "sports and adventure", Specialty: "sport", Color: "#FF9F00", Style: "lwr",
			Persona: "You are warm, bold and playful; you trust your runners and take risks when the crowd roars."},
		{ID: "llebeig", Name: "Llebeig", Role: "news and research", Specialty: "broadcasting", Color: "#0B6E4F", Style: "pob",
			Persona: "You are an excitable TV commentator with a vivid turn of phrase, but you never invent facts."},
	}
}

// memberByID busca un membre (un de buit amb aquest id si no hi és).
func memberByID(ms []Member, id string) Member {
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	return Member{ID: id, Name: id}
}

// identity és la presentació d'un membre per als prompts del model.
func (m Member) identity() string {
	var b strings.Builder
	b.WriteString("You are " + m.Name)
	if r := strings.TrimSpace(m.Role); r != "" {
		b.WriteString(" (" + r + ")")
	}
	b.WriteString(". ")
	if p := strings.TrimSpace(m.Persona); p != "" {
		b.WriteString(p)
	}
	return b.String()
}

// leagueStars converteix els membres en el que el motor de la lliga
// necessita per repartir-los entre els equips.
func leagueStars(ms []Member) []football.Wind {
	out := make([]football.Wind, 0, len(ms))
	for _, m := range ms {
		out = append(out, football.Wind{ID: m.ID, Name: m.Name, Role: m.Role, Specialty: m.Specialty, Color: m.Color, Style: m.Style})
	}
	return out
}
