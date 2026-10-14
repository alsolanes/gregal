package agent

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Memòria automàtica amb higiene. En acabar un torn de codi que ha fet
// feina de debò, el model pot proposar fins a dues notes duradores del
// projecte (convencions apreses, comandes que funcionen, paranys). Mai no
// s'escriuen soles: el client les ensenya («Remember this?») i només les
// que l'usuari accepta van a .gregal/memory.md amb config.Note. La
// consolidació (TidyMemory) proposa una llista nova sencera i el client
// n'ensenya el diff abans d'escriure-la.

// MaxMemorySuggestions topa les notes proposades per torn.
const MaxMemorySuggestions = 2

const maxNoteChars = 240

// MinExecsForMemory és el mínim d'eines executades perquè un torn valgui
// una proposta: un torn de dues lectures no ha après res durador i la
// crida extra seria cost per res.
const MinExecsForMemory = 3

// ShouldSuggestMemory diu si un torn acabat mereix una proposta de notes:
// mode de codi, prou eines, almenys una edició aplicada i acabat pel model
// (no per esgotament ni per error: un torn tallat no ha après res fiable).
func ShouldSuggestMemory(mode string, execs int, cutShort bool, hist []llm.Message) bool {
	return !cutShort && (mode == ModeCode || mode == ModeAutonomous || mode == ModeDebug) && execs >= MinExecsForMemory && SuccessfulEdits(hist) > 0
}

// SuccessfulEdits compta les escriptures (write/edit/patch) que han anat bé.
func SuccessfulEdits(hist []llm.Message) int {
	n := 0
	for _, m := range hist {
		if m.Role != "tool" || (m.Name != "write" && m.Name != "edit" && m.Name != "patch") {
			continue
		}
		out := strings.TrimSpace(m.Content)
		if strings.HasPrefix(out, "ERROR") || strings.HasPrefix(out, "EINA BLOQUEJADA") || strings.HasPrefix(out, "TOOL ") || strings.HasPrefix(out, "BLOCKED") {
			continue
		}
		n++
	}
	return n
}

const suggestSystem = `You maintain the durable project notes of a coding agent. From the work session below, propose AT MOST 2 notes worth remembering in future sessions on this project: conventions learned, commands that work (build, test, lint), non-obvious gotchas. Rules: each note is one standalone line under 200 characters; factual and reusable, not about this specific task; never secrets, tokens, personal data or temporary state; skip anything already in the existing notes. Most sessions teach nothing durable: then return an empty list. Reply with exactly one JSON object: {"notes": ["..."]}.`

const tidySystem = `You consolidate the durable project notes of a coding agent. Input: one note per line, format "- YYYY-MM-DD: text". Merge duplicates and near-duplicates into one line (keep the newest date), drop notes that are contradicted by a newer note or clearly stale, and keep every other note with its wording and date unchanged. Never invent new facts. Reply with exactly one JSON object: {"notes": ["- YYYY-MM-DD: text", ...]}.`

// SuggestMemory demana al model les notes duradores d'un torn.
func SuggestMemory(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, r config.Role, hist []llm.Message, existing []string) ([]string, error) {
	digest := TranscriptDigest(hist, 16000)
	if strings.TrimSpace(digest) == "" {
		return nil, nil
	}
	known := "(none)"
	if len(existing) > 0 {
		known = strings.Join(existing, "\n")
	}
	msgs := []llm.Message{
		{Role: "system", Content: suggestSystem},
		{Role: "user", Content: "EXISTING NOTES:\n" + known + "\n\nWORK SESSION:\n" + digest},
	}
	out, err := sideCall(ctx, client, cfg, p, r, msgs)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Notes []string `json:"notes"`
	}
	if err := sideJSON(out, &parsed); err != nil {
		return nil, err
	}
	return FilterMemorySuggestions(parsed.Notes, existing), nil
}

var notePrefixRe = regexp.MustCompile(`^-\s*\d{4}-\d{2}-\d{2}:\s*`)

// noteKey normalitza una nota per comparar-la: sense data, minúscules i
// sense puntuació repetida.
func noteKey(note string) string {
	note = notePrefixRe.ReplaceAllString(strings.TrimSpace(note), "")
	note = strings.TrimPrefix(note, "- ")
	note = strings.ToLower(strings.Join(strings.Fields(note), " "))
	return strings.TrimRight(note, ".!; ")
}

// FilterMemorySuggestions neteja les propostes: una línia, curta, sense
// duplicats (entre elles i amb les existents) i com a molt dues.
func FilterMemorySuggestions(notes, existing []string) []string {
	seen := map[string]bool{}
	for _, e := range existing {
		if k := noteKey(e); k != "" {
			seen[k] = true
		}
	}
	var out []string
	for _, n := range notes {
		n = strings.Join(strings.Fields(n), " ")
		n = strings.TrimPrefix(n, "- ")
		if n == "" || len([]rune(n)) > maxNoteChars {
			continue
		}
		k := noteKey(n)
		dup := seen[k]
		for s := range seen {
			if len(k) >= 12 && len(s) >= 12 && (strings.Contains(s, k) || strings.Contains(k, s)) {
				dup = true
			}
		}
		if dup {
			continue
		}
		seen[k] = true
		out = append(out, n)
		if len(out) == MaxMemorySuggestions {
			break
		}
	}
	return out
}

// TidyMemory proposa la llista consolidada de notes. No escriu res.
func TidyMemory(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, r config.Role, lines []string) ([]string, error) {
	if len(lines) == 0 {
		return nil, errors.New("no project notes to tidy")
	}
	msgs := []llm.Message{
		{Role: "system", Content: tidySystem},
		{Role: "user", Content: strings.Join(lines, "\n")},
	}
	out, err := sideCall(ctx, client, cfg, p, r, msgs)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Notes []string `json:"notes"`
	}
	if err := sideJSON(out, &parsed); err != nil {
		return nil, err
	}
	return NormalizeTidy(parsed.Notes, len(lines)), nil
}

// NormalizeTidy posa cada línia en el format del fitxer i rebutja que la
// consolidació en tingui més que l'original (merge i poda, mai afegir).
func NormalizeTidy(notes []string, max int) []string {
	today := time.Now().Format("2006-01-02")
	var out []string
	seen := map[string]bool{}
	for _, n := range notes {
		n = strings.Join(strings.Fields(n), " ")
		if n == "" {
			continue
		}
		if !notePrefixRe.MatchString(n) {
			n = "- " + today + ": " + strings.TrimPrefix(n, "- ")
		}
		if k := noteKey(n); seen[k] {
			continue
		} else {
			seen[k] = true
		}
		out = append(out, n)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

// NotesDiff és el diff línia a línia entre dues llistes de notes, amb
// «-» per a les que marxen, «+» per a les que entren i «  » per a les que
// queden. Les llistes són curtes: LCS directe.
func NotesDiff(before, after []string) string {
	n, m := len(before), len(after)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if strings.TrimSpace(before[i]) == strings.TrimSpace(after[j]) {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var b strings.Builder
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && strings.TrimSpace(before[i]) == strings.TrimSpace(after[j]):
			b.WriteString("  " + strings.TrimSpace(before[i]) + "\n")
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			b.WriteString("+ " + strings.TrimSpace(after[j]) + "\n")
			j++
		default:
			b.WriteString("- " + strings.TrimSpace(before[i]) + "\n")
			i++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
