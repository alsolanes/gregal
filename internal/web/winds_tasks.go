package web

import (
	"encoding/json"
	"gregal/internal/team"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	var mu sync.Mutex
	return func(e team.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Type == "done" || e.Type == "failed" || e.Type == "cancelled" {
			record.Status = e.Type
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
func (s *Server) handleWindTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	entries, err := os.ReadDir(filepath.Join(s.windStateDir(), "wind-tasks"))
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "could not read work history", 500)
		return
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
	writeJSON(w, map[string]any{"tasks": records})
}
