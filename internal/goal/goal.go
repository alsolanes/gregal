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
// Torna ok=false si no hi ha bloc o si és buit. El títol surt de la clau
// `task` (la del prompt actual) o `tasca` (objectius escrits abans que el
// prompt passés a l'anglès).
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
		if clau := strings.TrimSpace(k); strings.EqualFold(clau, "task") || strings.EqualFold(clau, "tasca") {
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

// Prompt és el system prompt afegit en mode objectiu. Les claus del bloc
// són en anglès (task, context, criteria, steps, risks); Parse accepta
// també les antigues en català.
func Prompt() string {
	return "\nGOAL MODE: DO NOT EXECUTE ANYTHING. Help the user pin down what they want before any work starts.\n" +
		"1. If the request is ambiguous or information is missing (scope, success criteria, what must NOT be touched), ask with the question tool (2-4 short options + free text): it is shown as a selectable choice and you can continue in the same turn. No plain-text interrogations (numbered lists of questions): one question per turn, with the tool. Do not invent anything.\n" +
		"2. You may read and explore the project (read, grep, glob) to ask informed questions; do not write or modify any file.\n" +
		"3. When you have enough context, reply with a block exactly like this:\n" +
		"```goal\n" +
		"task: <one sentence>\n" +
		"context: <why, and with what>\n" +
		"criteria:\n- <how we will know it is done right>\n" +
		"steps:\n- <planned steps>\n" +
		"risks:\n- <what could go wrong>\n" +
		"```\n" +
		"4. After the block, ask whether to execute it and mention that it can be edited.\n" +
		"End every reply either with a question (question tool) or with the ```goal block, never with plain text that neither asks nor closes anything."
}

// Task converteix un objectiu en la tasca que s'envia a l'agent en executar-lo.
func Task(g Goal) string {
	var b strings.Builder
	b.WriteString("Carry out this goal completely:\n\n")
	b.WriteString(g.Body)
	b.WriteString("\n\nProject: " + g.Project)
	if g.CWD != "" {
		b.WriteString(" (" + g.CWD + ")")
	}
	b.WriteString("\nMake the necessary changes, verify them, and summarize what you did and how you checked it.")
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
