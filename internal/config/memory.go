package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	errRecordaDuplicat = errors.New("ja ho tens apuntat")
	errSenseProjecte   = errors.New("sense directori de projecte")
)

// UserMemory: memòria persistent entre sessions (~/.config/gregal/memory.md).
// L'usuari hi desa fets estables (preferències, context, decisions) amb
// /remember; el sistema la injecta a cada system prompt. L'agent no hi
// escriu sol (B1 explícit; res d'inferències silencioses).
const MaxUserMemoryChars = 2000

// UserMemoryPath retorna el fitxer (GREGAL_MEMORY o ~/.config/gregal/memory.md).
func UserMemoryPath() string {
	if p := strings.TrimSpace(os.Getenv("GREGAL_MEMORY")); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "memory.md"
	}
	return filepath.Join(home, ".config", "gregal", "memory.md")
}

// LoadUserMemory llegeix la memòria ("" si no n'hi ha). Retalla a 2000
// caràcters: és context de totes les sessions, ha de ser dens.
func LoadUserMemory() string {
	raw, err := os.ReadFile(UserMemoryPath())
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	if len([]rune(text)) > MaxUserMemoryChars {
		text = string([]rune(text)[:MaxUserMemoryChars]) + "\n…(retallat)"
	}
	return text
}

// userMemoryBlock el compon SystemPrompt (buit si no hi ha memòria).
func userMemoryBlock() string {
	if m := LoadUserMemory(); m != "" {
		return "\n\nMEMÒRIA D'USUARI (fets estables entre sessions, prioritat alta):\n" + m
	}
	return ""
}

// Nucli compartit per les dues memòries (usuari i projecte): fitxer de
// línies "- <data>: <text>". Totes rebutgen buits i duplicats exactes.
func readEntries(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func appendEntry(path, text, usage string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New(usage)
	}
	lines := readEntries(path)
	entry := "- " + time.Now().Format("2006-01-02") + ": " + strings.ReplaceAll(text, "\n", " ")
	for _, l := range lines {
		if strings.TrimSuffix(l, " ") == entry {
			return errRecordaDuplicat
		}
	}
	lines = append(lines, entry)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func dropEntries(path, term, usage string) (int, error) {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return 0, errors.New(usage)
	}
	lines := readEntries(path)
	if lines == nil {
		return 0, nil
	}
	var keep []string
	dropped := 0
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), term) {
			dropped++
			continue
		}
		keep = append(keep, l)
	}
	if dropped == 0 {
		return 0, nil
	}
	body := ""
	if len(keep) > 0 {
		body = strings.Join(keep, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return 0, err
	}
	return dropped, nil
}

// Remember afegeix una línia a la memòria d'usuari.
func Remember(text string) error {
	return appendEntry(UserMemoryPath(), text, "res a recordar: /remember <text>")
}

// Recall torna les línies d'usuari que contenen el terme (buit = totes).
func Recall(term string) []string {
	return filterEntries(readEntries(UserMemoryPath()), term)
}

// Forget esborra línies d'usuari que contenen el terme i torna quantes.
func Forget(term string) (int, error) {
	return dropEntries(UserMemoryPath(), term, "què oblidem? /forget <terme>")
}

func filterEntries(lines []string, term string) []string {
	var out []string
	term = strings.ToLower(strings.TrimSpace(term))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if term == "" || strings.Contains(strings.ToLower(l), term) {
			out = append(out, l)
		}
	}
	return out
}

// Projecte: notes persistents a <cwd>/.gregal/memory.md (equip/agent).
// Telegram queda fora: el cwd del bot no és cap projecte (nota honesta al
// pla). TUI/headless/web les carreguen soles pel cwd de cada sessió.

// ProjectMemoryPath retorna el fitxer de notes del projecte ("" si cwd buit).
func ProjectMemoryPath(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return ""
	}
	return filepath.Join(cwd, ".gregal", "memory.md")
}

// Note afegeix una línia a les notes del projecte.
func Note(cwd, text string) error {
	p := ProjectMemoryPath(cwd)
	if p == "" {
		return errSenseProjecte
	}
	return appendEntry(p, text, "res a anotar: /nota <text>")
}

// Notes torna les línies del projecte que contenen el terme (buit = totes).
func Notes(cwd, term string) []string {
	if p := ProjectMemoryPath(cwd); p != "" {
		return filterEntries(readEntries(p), term)
	}
	return nil
}

// ForgetNote esborra línies del projecte que contenen el terme.
func ForgetNote(cwd, term string) (int, error) {
	p := ProjectMemoryPath(cwd)
	if p == "" {
		return 0, errSenseProjecte
	}
	return dropEntries(p, term, "quina nota oblidem? /oblida-nota <terme>")
}

// LoadProjectNotes llegeix les notes en brut per injectar al prompt.
func LoadProjectNotes(cwd string) string {
	p := ProjectMemoryPath(cwd)
	if p == "" {
		return ""
	}
	return strings.TrimSpace(strings.Join(readEntries(p), "\n"))
}
