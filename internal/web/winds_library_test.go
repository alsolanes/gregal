package web

import (
	"context"
	"encoding/json"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/runs"
	"gregal/internal/skills"
	"gregal/internal/team"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindLibraryArchiveRestoreAndMerge(t *testing.T) {
	s, _ := companionTestServer(t)
	for _, name := range []string{"Marinada", "Mestral"} {
		if _, err := s.winds.Save(companion.Wind{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := companion.ReplaceWindMemory(s.windStateDir(), "marinada", "- Prefers tea.\n"); err != nil {
		t.Fatal(err)
	}
	if err := companion.ReplaceWindBrain(s.windStateDir(), "marinada", "A useful source.\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveWindSkill("marinada", skills.Skill{Nom: "guide", Titol: "Guide", Descripcio: "When researching", Cos: "Check sources."}); err != nil {
		t.Fatal(err)
	}
	if err := s.archiveWind("marinada"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.winds.ByID("marinada"); ok {
		t.Fatal("archive still active")
	}
	items, err := s.archivedWinds()
	if err != nil || len(items) != 1 {
		t.Fatalf("archives: %v %v", items, err)
	}
	source := items[0]
	if source.Memory != "- Prefers tea.\n" || source.Knowledge != "A useful source.\n" || len(source.Skills) != 1 {
		t.Fatalf("incomplete archive: %+v", source)
	}
	if err := companion.ReplaceWindMemory(s.windStateDir(), "mestral", "- Existing fact.\n"); err != nil {
		t.Fatal(err)
	}
	target, _ := s.winds.ByID("mestral")
	if err := s.applyWindInformation(source, target, true, true); err != nil {
		t.Fatal(err)
	}
	if err := s.applyWindInformation(source, target, true, true); err != nil {
		t.Fatal(err)
	}
	memory := companion.LoadWindMemory(s.windStateDir(), "mestral")
	if strings.Count(memory, "Prefers tea") != 1 || !strings.Contains(memory, "Existing fact") {
		t.Fatalf("merge lost or duplicated facts: %q", memory)
	}
	if len(s.windSkills("mestral")) != 1 {
		t.Fatal("knowledge documents not transferred")
	}
	rec := postJSON(t, s.handleWindLibrary, `{"action":"restore","source":"`+source.ID+`"}`)
	if rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := s.winds.ByID("marinada"); !ok {
		t.Fatal("original identity not restored")
	}
	if rec = postJSON(t, s.handleWindLibrary, `{"action":"restore","source":"`+source.ID+`"}`); rec.Code != 409 {
		t.Fatal("restore overwrote active identity")
	}
}

func TestWindLibraryConflictLeavesDestinationUntouched(t *testing.T) {
	s, _ := companionTestServer(t)
	target, err := s.winds.Save(companion.Wind{Name: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	if err := companion.ReplaceWindMemory(s.windStateDir(), target.ID, "Existing"); err != nil {
		t.Fatal(err)
	}
	sk := skills.Skill{Nom: "guide", Descripcio: "Useful", Cos: "Existing document"}
	if err := s.saveWindSkill(target.ID, sk); err != nil {
		t.Fatal(err)
	}
	sk.Cos = "Conflicting document"
	if err := s.applyWindInformation(windLibraryItem{ID: "source", Memory: "Incoming", Skills: []skills.Skill{sk}}, target, true, true); err == nil {
		t.Fatal("conflicting document was overwritten")
	}
	if got := companion.LoadWindMemory(s.windStateDir(), target.ID); got != "Existing" {
		t.Fatalf("memory changed on failure: %q", got)
	}
	if got := s.windSkills(target.ID)[0].Cos; !strings.Contains(got, "Existing document") {
		t.Fatalf("document changed: %q", got)
	}
}

func TestWindKnowledgeMergePreservesCodeAndCase(t *testing.T) {
	existing := "Existing explanation.\n"
	incoming := "```js\nif (x) {\n  value = 'A';\n}\nif (y) {\n  value = 'a';\n}\n```\n"
	merged := mergeWindText(existing, incoming)
	if !strings.Contains(merged, incoming) || !strings.HasPrefix(merged, existing) {
		t.Fatal("merge changed document structure")
	}
	if twice := mergeWindText(merged, incoming); twice != merged {
		t.Fatal("repeated assignment duplicated the document")
	}
}

func TestWindSquadsAreUsefulAndIdempotent(t *testing.T) {
	s, _ := companionTestServer(t)
	for i := 0; i < 2; i++ {
		rec := postJSON(t, s.handleWindSquads, `{"id":"data","lang":"en"}`)
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
	}
	winds, err := s.winds.List()
	if err != nil || len(winds) != 5 {
		t.Fatalf("activation duplicated specialists: %d %v", len(winds), err)
	}
	for _, w := range winds {
		if w.Squad != "data" || w.Specialty == "" || len(w.Persona) < 70 {
			t.Fatalf("incomplete definition: %+v", w)
		}
	}
}

func TestWindTasksPersistPublicResultAndFolder(t *testing.T) {
	s, _ := companionTestServer(t)
	flow := &team.FlowSpec{Nodes: []team.FlowNode{{ID: "builder", Role: "builder", Label: "Tramuntana"}, {ID: "reviewer", Role: "reviewer"}}}
	record, err := s.recordWindTask("Build a website", s.cwd, "thread-one", flow)
	if err != nil {
		t.Fatal(err)
	}
	record(team.Event{Type: "tool_result", Activity: "write", Path: "index.html"})
	record(team.Event{Type: "done", Output: "Created the website"})
	// A new reader must see the result from disk, not a browser-local cache.
	rec := httptest.NewRecorder()
	s.handleWindTasks(rec, httptest.NewRequest(http.MethodGet, "/api/winds/tasks", nil))
	var response struct {
		Tasks []windTaskRecord `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Tasks) != 1 || response.Tasks[0].Thread != "thread-one" || response.Tasks[0].Status != "done" || response.Tasks[0].Events[0].Path != "index.html" || response.Tasks[0].Agents["builder"] != "Tramuntana" || len(response.Tasks[0].Agents) != 1 {
		t.Fatalf("lost record: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(s.windStateDir(), "wind-tasks", response.Tasks[0].ID+".json")); err != nil {
		t.Fatal(err)
	}
}

func TestWindLibraryUserIsolation(t *testing.T) {
	s, _ := companionTestServer(t)
	s.user = &User{Name: "one"}
	w, err := s.winds.Save(companion.Wind{Name: "Source"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.saveWindSnapshot(w, "archived"); err != nil {
		t.Fatal(err)
	}
	s.user = &User{Name: "two"}
	items, err := s.archivedWinds()
	if err != nil || len(items) != 0 {
		t.Fatalf("another user can see archives: %+v %v", items, err)
	}
}

func TestWindRoutineUsesCurrentMemoryAndKeepsPrivateHistory(t *testing.T) {
	s, _ := companionTestServer(t)
	store, err := jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.jobs = store
	wind, err := s.winds.Save(companion.Wind{Name: "Marinada", Persona: "Compare carefully."})
	if err != nil {
		t.Fatal(err)
	}
	if err := companion.ReplaceWindMemory(s.windStateDir(), wind.ID, "Current preference: tea."); err != nil {
		t.Fatal(err)
	}
	if err := companion.ReplaceWindBrain(s.windStateDir(), wind.ID, "Check the price per kilogram."); err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 4)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		requests <- string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Verified comparison."}}]}`))
	}))
	defer provider.Close()
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	_, err = s.executeJob(context.Background(), jobs.Job{ID: "routine", Name: "Compare", Prompt: "Compare tea", Wind: wind.ID}, runs.Run{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-requests:
		if !strings.Contains(request, "Current preference: tea.") || !strings.Contains(request, "price per kilogram") {
			t.Fatalf("routine omitted current wind information: %s", request)
		}
	default:
		t.Fatal("routine did not invoke the provider")
	}
	if err := store.AddRun(jobs.Run{ID: "private-run", JobID: "deleted-private-job", Wind: "wind-id", WindOwner: "anna", Output: "PRIVATE"}); err != nil {
		t.Fatal(err)
	}
	// La rutina deixa feina de fons (aprenentatge del vent) que llegeix s.user:
	// cal que acabi abans de canviar d'usuari, o el detector de curses salta.
	waitBackground()
	s.user = &User{Name: "bert"}
	rec := httptest.NewRecorder()
	s.handleJobRuns(rec, httptest.NewRequest(http.MethodGet, "/api/jobs/runs", nil))
	if strings.Contains(rec.Body.String(), "PRIVATE") {
		t.Fatal("deleted routine leaked private history")
	}
}
