package companion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// La memòria d'un vent sobre l'usuari: gustos, preferències i fets de
// converses anteriors. El cervell (windbrain) guarda fets del tema amb
// fonts; la memòria guarda fets de la persona, sense cites. Mateix
// format (markdown, bala per fet) i mateixes garanties: llegible,
// editable a mà i amb revisions perquè una extracció vella no ressusciti
// el que l'usuari ha esborrat.
//
//	<windID>.md dins wind-memory/
//
//	- A l'Aleix li agrada el cafè curt.
//	- No vol paraulotes als missatges.

const MaxWindMemoryBytes = 8 * 1024

var memoryMu sync.Mutex
var memoryRevisions = map[string]uint64{}

func memoryRevisionKey(stateDir, windID string) string {
	path := memoryPath(stateDir, windID)
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return filepath.Clean(path)
}

// WindMemoryRevision captura la generació d'edicions manuals.
func WindMemoryRevision(stateDir, windID string) uint64 {
	memoryMu.Lock()
	defer memoryMu.Unlock()
	return memoryRevisions[memoryRevisionKey(stateDir, windID)]
}

// MemoryDir és la carpeta de les memòries dels vents.
func MemoryDir(stateDir string) string { return filepath.Join(stateDir, "wind-memory") }

func memoryPath(stateDir, windID string) string {
	return filepath.Join(MemoryDir(stateDir), slug(windID)+".md")
}

// LoadWindMemory torna el que el vent recorda de l'usuari ("" si res).
func LoadWindMemory(stateDir, windID string) string {
	raw, err := os.ReadFile(memoryPath(stateDir, windID))
	if err != nil {
		return ""
	}
	return string(raw)
}

// ReplaceWindMemory substitueix la memòria sencera (inclòs buidar-la).
func ReplaceWindMemory(stateDir, windID, text string) error {
	if windID == "" || slug(windID) != windID {
		return fmt.Errorf("invalid wind id")
	}
	if len(text) > MaxWindMemoryBytes {
		return fmt.Errorf("memory exceeds %d bytes", MaxWindMemoryBytes)
	}
	memoryMu.Lock()
	defer memoryMu.Unlock()
	if err := writeWindMemory(stateDir, windID, text); err != nil {
		return err
	}
	memoryRevisions[memoryRevisionKey(stateDir, windID)]++
	return nil
}

func writeWindMemory(stateDir, windID, text string) error {
	if err := os.MkdirAll(MemoryDir(stateDir), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(MemoryDir(stateDir), ".memory-*.tmp")
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
	return os.Rename(f.Name(), memoryPath(stateDir, windID))
}

// WindMemoryNotes torna la memòria com a línies.
func WindMemoryNotes(stateDir, windID string) []string {
	raw := LoadWindMemory(stateDir, windID)
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

// AppendWindMemory afegeix fets nous (sense duplicats, poda per davant).
func AppendWindMemory(stateDir, windID string, facts []string) error {
	memoryMu.Lock()
	defer memoryMu.Unlock()
	return appendWindMemory(stateDir, windID, facts)
}

// AppendWindMemoryIfRevision no ressuscita el que l'usuari ha editat.
func AppendWindMemoryIfRevision(stateDir, windID string, revision uint64, facts []string) error {
	memoryMu.Lock()
	defer memoryMu.Unlock()
	if memoryRevisions[memoryRevisionKey(stateDir, windID)] != revision {
		return nil
	}
	return appendWindMemory(stateDir, windID, facts)
}

func appendWindMemory(stateDir, windID string, facts []string) error {
	var fresh []string
	for _, f := range facts {
		if f = strings.TrimSpace(f); f != "" {
			fresh = append(fresh, f)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, n := range WindMemoryNotes(stateDir, windID) {
		known[strings.ToLower(n)] = true
	}
	var lines []string
	for _, n := range WindMemoryNotes(stateDir, windID) {
		lines = append(lines, "- "+n)
	}
	for _, f := range fresh {
		if !known[strings.ToLower(f)] {
			lines = append(lines, "- "+f)
			known[strings.ToLower(f)] = true
		}
	}
	text := strings.Join(lines, "\n") + "\n"
	for len(text) > MaxWindMemoryBytes && len(lines) > 1 {
		lines = lines[1:]
		text = strings.Join(lines, "\n") + "\n"
	}
	return writeWindMemory(stateDir, windID, text)
}
