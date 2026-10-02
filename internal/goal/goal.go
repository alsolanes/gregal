// Package goal implementa el mode objectiu: l'agent ajuda a concretar què es
// vol fer, ho desa com a objectiu estructurat i després es pot executar.
package goal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Estats d'un objectiu.
const (
	StatusEsborrany = "esborrany"
	StatusFet       = "fet"
)

// Goal és un objectiu concretat amb l'agent.
type Goal struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Project string    `json:"project"`
	CWD     string    `json:"cwd"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Status  string    `json:"status"`
}

const (
	obertura  = "```goal"
	tancament = "```"
)

// Parse extreu un objectiu del bloc ```goal de la resposta de l'agent.
// Torna ok=false si no hi ha bloc o si és buit.
func Parse(reply, cwd string) (Goal, bool) {
	cos, ok := blocGoal(reply)
	if !ok {
		return Goal{}, false
	}
	titol := ""
	for _, linia := range strings.Split(cos, "\n") {
		k, v, trobat := strings.Cut(strings.TrimSpace(linia), ":")
		if !trobat {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "tasca") {
			titol = strings.TrimSpace(v)
			break
		}
	}
	if titol == "" {
		// Sense tasca, el primer text no buit fa de títol.
		for _, linia := range strings.Split(cos, "\n") {
			if l := strings.TrimSpace(linia); l != "" {
				titol = l
				break
			}
		}
	}
	if titol == "" {
		return Goal{}, false
	}
	return Goal{
		ID:      newID(),
		Created: time.Now().UTC(),
		Project: filepath.Base(strings.TrimRight(cwd, string(filepath.Separator))),
		CWD:     cwd,
		Title:   titol,
		Body:    cos,
		Status:  StatusEsborrany,
	}, true
}

// blocGoal retorna el contingut del primer bloc ```goal, sense les tanques.
func blocGoal(reply string) (string, bool) {
	i := strings.Index(reply, obertura)
	if i < 0 {
		return "", false
	}
	resta := reply[i+len(obertura):]
	j := strings.Index(resta, tancament)
	if j < 0 {
		// Bloc sense tancar (resposta tallada): aprofitem el que hi ha.
		j = len(resta)
	}
	cos := strings.TrimSpace(resta[:j])
	if cos == "" {
		return "", false
	}
	return cos, true
}

func newID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%06x", time.Now().UnixNano()&0xffffff)
	}
	return hex.EncodeToString(b)
}

// Path retorna el fitxer d'objectius dins del directori donat.
func Path(dir string) string { return filepath.Join(dir, "goals.json") }

type disc struct {
	Goals []Goal `json:"goals"`
}

// Save desa (o actualitza) un objectiu dins del directori donat.
func Save(dir string, g Goal) error {
	if strings.TrimSpace(g.ID) == "" {
		return errors.New("objectiu sense id")
	}
	llista, err := llegir(dir)
	if err != nil {
		return err
	}
	if g.Status == "" {
		g.Status = StatusEsborrany
	}
	trobat := false
	for i := range llista {
		if llista[i].ID == g.ID {
			llista[i] = g
			trobat = true
			break
		}
	}
	if !trobat {
		llista = append(llista, g)
	}
	return escriure(dir, llista)
}

// List retorna els objectius, els més nous primer. project buit = tots.
func List(dir, project string) ([]Goal, error) {
	llista, err := llegir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Goal, 0, len(llista))
	for _, g := range llista {
		if project == "" || g.Project == project {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Created.Equal(out[j].Created) {
			return out[i].ID > out[j].ID
		}
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
}

// Get retorna un objectiu per id.
func Get(dir, id string) (Goal, error) {
	llista, err := llegir(dir)
	if err != nil {
		return Goal{}, err
	}
	for _, g := range llista {
		if g.ID == id {
			return g, nil
		}
	}
	return Goal{}, fmt.Errorf("no existeix cap objectiu amb id %s", id)
}

// Delete esborra un objectiu.
func Delete(dir, id string) error {
	llista, err := llegir(dir)
	if err != nil {
		return err
	}
	out := make([]Goal, 0, len(llista))
	for _, g := range llista {
		if g.ID == id {
			continue
		}
		out = append(out, g)
	}
	if len(out) == len(llista) {
		return fmt.Errorf("no existeix cap objectiu amb id %s", id)
	}
	return escriure(dir, out)
}

func llegir(dir string) ([]Goal, error) {
	b, err := os.ReadFile(Path(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var d disc
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d.Goals, nil
}

func escriure(dir string, llista []Goal) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(disc{Goals: llista}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// 0600: els objectius poden contenir detalls del projecte.
	return os.WriteFile(Path(dir), b, 0o600)
}

// Prompt és el system prompt afegit en mode objectiu.
func Prompt() string {
	return "\nMode objectiu: NO EXECUTIS RES. Ajuda a concretar què vol fer l'usuari abans de treballar-hi.\n" +
		"1. Si la petició és ambigua o falta informació (abast, criteri d'èxit, què NO tocar), pregunta amb l'eina question (2-4 opcions curtes + text lliure): surt seleccionable i pots continuar en el mateix torn. PROHIBIT l'interrogatori en text pla (llistes numerades): una pregunta per torn amb l'eina. No inventis res.\n" +
		"2. Pots llegir i explorar el projecte (read, grep, glob) per preguntar amb coneixement de causa; no escriguis ni modifiquis cap fitxer.\n" +
		"3. Quan tinguis prou context, respon amb un bloc exactament així:\n" +
		"```goal\n" +
		"tasca: <una frase>\n" +
		"context: <per què i amb què>\n" +
		"criteris:\n- <com se sabrà que està bé>\n" +
		"passos:\n- <passos previstos>\n" +
		"riscos:\n- <què pot fallar>\n" +
		"```\n" +
		"4. Després del bloc, pregunta si el vols executar i recorda que pot editar-se.\n" +
		"Acaba cada resposta o bé amb una pregunta (eina question) o bé amb el bloc ```goal: mai amb text pla que no demani ni tanqui res."
}

// Task converteix un objectiu en la tasca que s'envia a l'agent en executar-lo.
func Task(g Goal) string {
	var b strings.Builder
	b.WriteString("Executa aquest objectiu complet:\n\n")
	b.WriteString(g.Body)
	b.WriteString("\n\nProjecte: " + g.Project)
	if g.CWD != "" {
		b.WriteString(" (" + g.CWD + ")")
	}
	b.WriteString("\nFes els canvis necessaris, verifica'ls i resumeix què has fet i com ho has comprovat.")
	return b.String()
}

// Resum és una línia curta per a llistes de la UI.
func Resum(g Goal) string {
	estat := "·"
	if g.Status == StatusFet {
		estat = "✓"
	}
	return fmt.Sprintf("%s %s %s", estat, g.ID, g.Title)
}
