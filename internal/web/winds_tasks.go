package web

import (
	"encoding/json"
	"fmt"
	"gregal/internal/team"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Public work events are durable; model reasoning and tool arguments are not stored here.
type windTaskRecord struct {
	ID        string    `json:"id"`
	Task      string    `json:"task"`
	Workspace string    `json:"workspace"`
	Thread    string    `json:"thread,omitempty"`
	Created   time.Time `json:"created"`
	Status    string    `json:"status"`
	// Agents names each flow seat (builder → Tramuntana) so history reads like the live view.
	Agents map[string]string `json:"agents,omitempty"`
	Events []team.Event      `json:"events"`
}

func (s *Server) recordWindTask(task, workspace, thread string, flow *team.FlowSpec) (func(team.Event), error) {
	record := windTaskRecord{ID: windRecordID(), Task: task, Workspace: workspace, Thread: thread, Created: time.Now().UTC(), Status: "started"}
	if flow != nil {
		for _, node := range flow.Nodes {
			if node.ID != "" && node.Label != "" {
				if record.Agents == nil {
					record.Agents = map[string]string{}
				}
				record.Agents[node.ID] = node.Label
			}
		}
	}
	path := filepath.Join(s.windStateDir(), "wind-tasks", record.ID+".json")
	if err := writeWindJSON(path, record); err != nil {
		return nil, err
	}
	liveWindTasks.Store(record.ID, true)
	var mu sync.Mutex
	return func(e team.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Type == "done" || e.Type == "failed" || e.Type == "cancelled" {
			record.Status = e.Type
			liveWindTasks.Delete(record.ID)
		}
		if len(e.Output) > 80000 {
			e.Output = e.Output[:80000]
		}
		record.Events = append(record.Events, e)
		if len(record.Events) > 160 {
			record.Events = record.Events[len(record.Events)-160:]
		}
		_ = writeWindJSON(path, record)
	}, nil
}

// liveWindTasks són les feines que corren en aquest procés. Una feina que
// el disc diu «started» però que no hi és, va quedar a mitges quan el
// servidor es va reiniciar.
var liveWindTasks sync.Map

// windTaskSummary és una feina per a la llista del mòbil: sense el registre
// sencer d'esdeveniments (pot fer centenars de kB), però amb el que cal per
// saber com va i obrir-ne el resultat.
type windTaskSummary struct {
	ID        string            `json:"id"`
	Task      string            `json:"task"`
	Workspace string            `json:"workspace"`
	Thread    string            `json:"thread,omitempty"`
	Created   time.Time         `json:"created"`
	Updated   time.Time         `json:"updated"`
	Status    string            `json:"status"` // running | done | failed | cancelled | interrupted
	Agents    map[string]string `json:"agents,omitempty"`
	Files     []string          `json:"files"`
	Result    string            `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
}

func summarizeWindTask(rec windTaskRecord, updated time.Time) windTaskSummary {
	out := windTaskSummary{ID: rec.ID, Task: rec.Task, Workspace: rec.Workspace, Thread: rec.Thread,
		Created: rec.Created, Updated: updated, Status: rec.Status, Agents: rec.Agents, Files: []string{}}
	if out.Status == "started" {
		out.Status = "running"
		if _, live := liveWindTasks.Load(rec.ID); !live {
			out.Status = "interrupted"
		}
	}
	seen := map[string]bool{}
	for _, e := range rec.Events {
		if p := strings.TrimSpace(e.Path); p != "" && !seen[p] {
			// Els fitxers de prova que el constructor fa servir per
			// comprovar la seva feina (_harness.html) no són el resultat.
			if !strings.HasPrefix(filepath.Base(p), "_") {
				out.Files = append(out.Files, filepath.ToSlash(p))
			}
			seen[p] = true
		}
		switch e.Type {
		case "done":
			if t := strings.TrimSpace(e.Output); t != "" {
				out.Result = oneLineKeepBreaks(t, 600)
			}
		case "failed", "cancelled":
			if t := strings.TrimSpace(e.Message); t != "" {
				out.Error = oneLineKeepBreaks(t, 300)
			}
		}
	}
	return out
}

func (s *Server) handleWindTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	records, err := s.windTaskRecords()
	if err != nil {
		http.Error(w, "could not read work history", 500)
		return
	}
	if r.URL.Query().Get("summary") == "1" {
		out := s.windTaskSummaries(records)
		out = append(s.crewTasksInDiscussion(out), out...)
		writeJSON(w, map[string]any{"tasks": out})
		return
	}
	writeJSON(w, map[string]any{"tasks": records})
}

// windTaskRecords són les feines d'aquest usuari, de la més nova a la més
// vella (100 com a molt).
func (s *Server) windTaskRecords() ([]windTaskRecord, error) {
	entries, err := os.ReadDir(filepath.Join(s.windStateDir(), "wind-tasks"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	records := []windTaskRecord{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.windStateDir(), "wind-tasks", entry.Name()))
		if err != nil {
			continue
		}
		var record windTaskRecord
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		if s.user.Allow(record.Workspace) != nil {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Created.After(records[j].Created) })
	if len(records) > 100 {
		records = records[:100]
	}
	return records, nil
}

func (s *Server) windTaskSummaries(records []windTaskRecord) []windTaskSummary {
	out := make([]windTaskSummary, 0, len(records))
	for _, rec := range records {
		updated := rec.Created
		if info, err := os.Stat(filepath.Join(s.windStateDir(), "wind-tasks", rec.ID+".json")); err == nil {
			updated = info.ModTime().UTC()
		}
		out = append(out, summarizeWindTask(rec, updated))
	}
	return out
}

// crewTasksInDiscussion són les feines d'equip que encara són a la
// discussió de disseny: la feina no es desa fins que algú comença a
// construir, i abans el mòbil no veia res durant aquells minuts. Només les
// de les rotllanes d'aquest usuari.
func (s *Server) crewTasksInDiscussion(known []windTaskSummary) []windTaskSummary {
	enMarxa := map[string]bool{}
	for _, t := range known {
		if t.Thread != "" && t.Status == "running" {
			enMarxa[t.Thread] = true
		}
	}
	var out []windTaskSummary
	crewJobs.Range(func(k, v any) bool {
		id, _ := k.(string)
		job, _ := v.(*crewJob)
		if id == "" || job == nil || enMarxa[id] {
			return true
		}
		if step := job.get(); step != "discussing" && step != "waiting" {
			return true
		}
		rot, ok := s.rotllanaByID(id)
		if !ok {
			return true
		}
		names := map[string]string{}
		for i, wnd := range s.rotMembersOf(rot) {
			names[fmt.Sprintf("%02d", i)] = wnd.Name
		}
		out = append(out, windTaskSummary{ID: "crew-" + id, Task: rot.Name, Workspace: rot.Workspace, Thread: id,
			Created: time.Now().UTC(), Updated: time.Now().UTC(), Status: "running", Agents: names, Files: []string{}})
		return true
	})
	return out
}
