package companion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// El cervell d'un vent: una base de coneixement pròpia per tema que creix
// amb el que el vent aprèn de les seves cerques. És un markdown senzill
// (bala per fet) dins l'estat de l'usuari: llegible, editable i portàtil.
//
//	<windID>.md
//	- Els MacBook Air M4 tenen bateria de 53,8 Wh i pantalla de 13,6". [apple.com]
//
// El fitxer es poda per davant quan passa de maxBrainBytes: es queden els
// fets més recents.

const MaxWindKnowledgeBytes = 24 * 1024
const maxBrainBytes = MaxWindKnowledgeBytes

var brainMu sync.Mutex

// Only explicit replacements advance the revision. Independent extractions may
// merge, but none may restore knowledge superseded by a user's edit/forget.
var brainRevisions = map[string]uint64{}

func brainRevisionKey(stateDir, windID string) string {
	path := brainPath(stateDir, windID)
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return filepath.Clean(path)
}

// WindBrainRevision captures the manual-edit generation before scheduling work.
func WindBrainRevision(stateDir, windID string) uint64 {
	brainMu.Lock()
	defer brainMu.Unlock()
	return brainRevisions[brainRevisionKey(stateDir, windID)]
}

// BrainDir és la carpeta dels cervells dels vents.
func BrainDir(stateDir string) string { return filepath.Join(stateDir, "wind-knowledge") }

func brainPath(stateDir, windID string) string {
	return filepath.Join(BrainDir(stateDir), slug(windID)+".md")
}

// LoadWindBrain torna el que el vent ja sap del seu tema ("" si res).
func LoadWindBrain(stateDir, windID string) string {
	raw, err := os.ReadFile(brainPath(stateDir, windID))
	if err != nil {
		return ""
	}
	return string(raw)
}

// ReplaceWindBrain replaces the complete markdown (including empty text).
// It shares the append lock, and publishes only a complete file via rename.
func ReplaceWindBrain(stateDir, windID, text string) error {
	if windID == "" || slug(windID) != windID {
		return fmt.Errorf("invalid wind id")
	}
	if len(text) > MaxWindKnowledgeBytes {
		return fmt.Errorf("knowledge exceeds %d bytes", MaxWindKnowledgeBytes)
	}
	brainMu.Lock()
	defer brainMu.Unlock()
	if err := writeWindBrain(stateDir, windID, text); err != nil {
		return err
	}
	brainRevisions[brainRevisionKey(stateDir, windID)]++
	return nil
}

func writeWindBrain(stateDir, windID, text string) error {
	if err := os.MkdirAll(BrainDir(stateDir), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(BrainDir(stateDir), ".knowledge-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), brainPath(stateDir, windID))
}

// WindBrainNotes torna els fets del cervell com a línies.
func WindBrainNotes(stateDir, windID string) []string {
	raw := LoadWindBrain(stateDir, windID)
	if raw == "" {
		return nil
	}
	var out []string
	for _, l := range strings.Split(raw, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "- ") {
			out = append(out, strings.TrimPrefix(l, "- "))
		}
	}
	return out
}

// AppendWindBrain afegeix fets nous al cervell del vent. Els duplicats
// (mateixa línia) no s'hi posen dues vegades, i el fitxer es poda per
// davant si passa de mida: mana el més recent.
func AppendWindBrain(stateDir, windID string, facts []string) error {
	brainMu.Lock()
	defer brainMu.Unlock()
	return appendWindBrain(stateDir, windID, facts)
}

// AppendWindBrainIfRevision atomically checks that no manual edit has occurred
// since extraction began. A superseded extraction is intentionally a no-op.
func AppendWindBrainIfRevision(stateDir, windID string, revision uint64, facts []string) error {
	brainMu.Lock()
	defer brainMu.Unlock()
	if brainRevisions[brainRevisionKey(stateDir, windID)] != revision {
		return nil
	}
	return appendWindBrain(stateDir, windID, facts)
}

// Caller holds brainMu across reading, merging and publishing the new text.
func appendWindBrain(stateDir, windID string, facts []string) error {
	var fresh []string
	for _, f := range facts {
		f = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(f), "-"))
		if f == "" || len(f) < 8 {
			continue
		}
		if len([]rune(f)) > 400 {
			r := []rune(f)
			f = string(r[:400]) + "…"
		}
		fresh = append(fresh, "- "+f)
	}
	if len(fresh) == 0 {
		return nil
	}
	old := strings.TrimSpace(LoadWindBrain(stateDir, windID))
	var lines []string
	if old != "" {
		lines = strings.Split(old, "\n")
	}
	seen := map[string]bool{}
	for _, l := range lines {
		seen[strings.TrimSpace(l)] = true
	}
	for _, f := range fresh {
		if !seen[f] {
			lines = append(lines, f)
			seen[f] = true
		}
	}
	out := strings.Join(lines, "\n") + "\n"
	if len(out) > maxBrainBytes {
		cut := len(out) - maxBrainBytes
		if i := strings.IndexByte(out[cut:], '\n'); i >= 0 {
			cut += i + 1
		}
		out = out[cut:]
	}
	return writeWindBrain(stateDir, windID, out)
}

// WindBrainSystem afegeix el cervell al brief del vent: què ja sap.
func WindBrainSystem(brain string) string {
	if strings.TrimSpace(brain) == "" {
		return ""
	}
	return "\n\nWHAT YOU ALREADY KNOW ABOUT YOUR TOPIC (your own knowledge base, learned from past searches — trust it, cite it, and add to it):\n" + brain
}
