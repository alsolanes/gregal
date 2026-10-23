package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/skills"
)

// Archives belong to the user, independently of the active wind catalogue.
var windLibraryMu sync.Mutex

type windLibraryItem struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Wind      companion.Wind `json:"wind"`
	Memory    string         `json:"memory"`
	Knowledge string         `json:"knowledge"`
	Skills    []skills.Skill `json:"skills"`
	Created   time.Time      `json:"created"`
	Origin    string         `json:"origin,omitempty"`
}

func windRecordID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func writeWindJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".record-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), path)
}
func (s *Server) windArchiveDir() string {
	return filepath.Join(s.windStateDir(), "wind-library", "archives")
}
func (s *Server) windSnapshot(w companion.Wind, kind string) windLibraryItem {
	return windLibraryItem{ID: windRecordID(), Kind: kind, Wind: w, Memory: companion.LoadWindMemory(s.windStateDir(), w.ID), Knowledge: s.windBrain(w.ID), Skills: s.windSkills(w.ID), Created: time.Now().UTC()}
}
func (s *Server) saveWindSnapshot(w companion.Wind, kind string) (windLibraryItem, error) {
	item := s.windSnapshot(w, kind)
	return item, writeWindJSON(filepath.Join(s.windArchiveDir(), item.ID+".json"), item)
}
func (s *Server) archivedWinds() ([]windLibraryItem, error) {
	entries, err := os.ReadDir(s.windArchiveDir())
	if os.IsNotExist(err) {
		return []windLibraryItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []windLibraryItem{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.windArchiveDir(), e.Name()))
		if err != nil {
			return nil, err
		}
		var item windLibraryItem
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}
func (s *Server) windLibrary() ([]windLibraryItem, error) {
	winds, err := s.winds.List()
	if err != nil {
		return nil, err
	}
	archives, err := s.archivedWinds()
	if err != nil {
		return nil, err
	}
	out := []windLibraryItem{}
	seen := map[string]bool{}
	for _, w := range winds {
		item := s.windSnapshot(w, "active")
		item.ID = "active-" + w.ID
		out = append(out, item)
		seen[w.ID] = true
	}
	out = append(out, archives...)
	for _, item := range archives {
		if item.Kind == "archived" || item.Kind == "recovered" {
			seen[item.Wind.ID] = true
		}
	}
	// Recover orphaned legacy files left by catalogue-only deletion.
	for _, dir := range []string{companion.MemoryDir(s.windStateDir()), companion.BrainDir(s.windStateDir()), filepath.Join(s.windStateDir(), "wind-skills")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			id := e.Name()
			if !e.IsDir() {
				if !strings.HasSuffix(id, ".md") {
					continue
				}
				id = strings.TrimSuffix(id, ".md")
			}
			if id == "" || companion.SlugWind(id) != id || seen[id] {
				continue
			}
			seen[id] = true
			item := s.windSnapshot(companion.Wind{ID: id, Name: id, Color: "#C1B4DE", Style: "pso"}, "recovered")
			item.ID = "recovered-" + id
			out = append(out, item)
		}
	}
	return out, nil
}
func (s *Server) archiveWind(id string) error {
	windLibraryMu.Lock()
	defer windLibraryMu.Unlock()
	w, ok := s.winds.ByID(id)
	if !ok {
		return fmt.Errorf("unknown wind")
	}
	if _, err := s.saveWindSnapshot(w, "archived"); err != nil {
		return err
	}
	// Invalidate background extractions captured before the archive.
	if err := companion.ReplaceWindMemory(s.windStateDir(), id, companion.LoadWindMemory(s.windStateDir(), id)); err != nil {
		return err
	}
	if err := companion.ReplaceWindBrain(s.windStateDir(), id, s.windBrain(id)); err != nil {
		return err
	}
	return s.winds.Delete(id)
}
func mergeWindText(existing, incoming string) string {
	// Preserve prose, code blocks, case and repeated structural lines verbatim.
	// Reassigning the same complete block is idempotent; partial overlap is kept
	// rather than risking removal of information from either source.
	if strings.TrimSpace(incoming) == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return incoming
	}
	if strings.Contains(existing, strings.TrimSpace(incoming)) {
		return existing
	}
	return strings.TrimRight(existing, "\r\n") + "\n\n" + incoming
}

func (s *Server) applyWindInformation(source windLibraryItem, target companion.Wind, memory, knowledge bool) error {
	before := s.windSnapshot(target, "version")
	mem, brain := before.Memory, before.Knowledge
	if memory {
		mem = mergeWindText(mem, source.Memory)
	}
	if knowledge {
		brain = mergeWindText(brain, source.Knowledge)
	}
	if len(mem) > companion.MaxWindMemoryBytes || len(brain) > companion.MaxWindKnowledgeBytes {
		return fmt.Errorf("combined information is too large; select or edit the information first")
	}
	existing := map[string]skills.Skill{}
	for _, sk := range before.Skills {
		existing[sk.Nom] = sk
	}
	if knowledge {
		for _, sk := range source.Skills {
			if !validWindSkillName(sk.Nom) {
				return fmt.Errorf("invalid knowledge name")
			}
			if old, ok := existing[sk.Nom]; ok && (old.Cos != sk.Cos || old.Descripcio != sk.Descripcio) {
				return fmt.Errorf("knowledge conflict: %s; rename it before assigning", sk.Nom)
			}
		}
	}
	before.Origin = source.ID
	if err := writeWindJSON(filepath.Join(s.windArchiveDir(), before.ID+".json"), before); err != nil {
		return err
	}
	created := []string{}
	rollback := func() {
		_ = companion.ReplaceWindMemory(s.windStateDir(), target.ID, before.Memory)
		_ = companion.ReplaceWindBrain(s.windStateDir(), target.ID, before.Knowledge)
		for _, p := range created {
			_ = os.Remove(p)
		}
	}
	if err := companion.ReplaceWindMemory(s.windStateDir(), target.ID, mem); err != nil {
		return err
	}
	if err := companion.ReplaceWindBrain(s.windStateDir(), target.ID, brain); err != nil {
		rollback()
		return err
	}
	if knowledge {
		for _, sk := range source.Skills {
			if _, ok := existing[sk.Nom]; ok {
				continue
			}
			if err := s.saveWindSkill(target.ID, sk); err != nil {
				rollback()
				return err
			}
			created = append(created, filepath.Join(s.windSkillsDir(target.ID), sk.Nom+".md"))
		}
	}
	return nil
}
func (s *Server) handleWindLibrary(w http.ResponseWriter, r *http.Request) {
	windLibraryMu.Lock()
	defer windLibraryMu.Unlock()
	items, err := s.windLibrary()
	if err != nil {
		http.Error(w, "could not read wind library", 500)
		return
	}
	if r.Method == http.MethodGet {
		if id := r.URL.Query().Get("export"); id != "" {
			for _, item := range items {
				if item.ID == id {
					w.Header().Set("Content-Disposition", "attachment; filename=wind-information.json")
					writeJSON(w, item)
					return
				}
			}
			http.Error(w, "unknown archive", 404)
			return
		}
		writeJSON(w, map[string]any{"items": items})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", 405)
		return
	}
	var req struct {
		Action    string `json:"action"`
		Source    string `json:"source"`
		Target    string `json:"target"`
		Memory    bool   `json:"memory"`
		Knowledge bool   `json:"knowledge"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	var source *windLibraryItem
	for i := range items {
		if items[i].ID == req.Source {
			source = &items[i]
			break
		}
	}
	if source == nil {
		http.Error(w, "unknown source", 404)
		return
	}
	switch req.Action {
	case "restore":
		if source.Kind != "archived" && source.Kind != "recovered" {
			http.Error(w, "only archived winds can be restored", 400)
			return
		}
		if _, ok := s.winds.ByID(source.Wind.ID); ok {
			http.Error(w, "a wind with this identity is active; assign its information instead", 409)
			return
		}
		// Files are retained on archive. Restore the captured contents as well as identity.
		if err = s.applyWindInformation(*source, source.Wind, true, true); err == nil {
			err = s.winds.Restore(source.Wind)
		}
	case "assign":
		target, ok := s.winds.ByID(req.Target)
		if !ok || (!req.Memory && !req.Knowledge) {
			http.Error(w, "choose a target and information", 400)
			return
		}
		if target.ID == source.Wind.ID {
			http.Error(w, "choose another wind", 400)
			return
		}
		err = s.applyWindInformation(*source, target, req.Memory, req.Knowledge)
	default:
		http.Error(w, "unknown action", 400)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
