package web

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/team"
)

func TestCrewJobMemoryChangesOnlyAfterAtomicReplace(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	jobPath := crewJobPath(s, id)
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(jobPath, 0o700); err != nil {
		t.Fatal(err)
	}
	job := &crewJob{path: jobPath, step: "working"}
	if err := job.set("done"); err == nil {
		t.Fatal("expected atomic replacement to fail against a directory")
	}
	if got := job.get(); got != "working" {
		t.Fatalf("memory status advanced after failed disk replacement: %q", got)
	}
}

func TestCrewTerminalResultSurvivesStatusFailureAndRecoversOnce(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	rot, ok := s.rotllanaByID(id)
	if !ok {
		t.Fatal("missing rotllana")
	}
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		t.Fatal("missing lead wind")
	}
	st := &crewState{Task: "finish this", RunID: "terminal-injected", Started: time.Now()}
	if err := saveCrewState(s, id, st); err != nil {
		t.Fatal(err)
	}
	job := &crewJob{path: crewJobPath(s, id)}
	if err := job.set("working"); err != nil {
		t.Fatal(err)
	}
	job.beforeReplace = func(step string) error {
		if step == "done" {
			return errors.New("injected terminal job write failure")
		}
		return nil
	}
	const result = "work is complete and verified"
	if err := s.finishCrewRun(rot, st, members[0], job, "done", result); err == nil {
		t.Fatal("expected terminal job write to fail")
	}
	if got := job.get(); got != "failed" {
		t.Fatalf("failed durable status not exposed in memory: %q", got)
	}
	pending, ok := loadCrewState(s, id)
	if !ok || pending.PendingResult == "" || pending.TerminalStep != "done" {
		t.Fatalf("terminal recovery checkpoint missing: %+v", pending)
	}
	if got := crewResultCount(s, rot, result); got != 1 {
		t.Fatalf("result history count before restart = %d", got)
	}

	// A fresh process has no injected fault. Recovery reuses the run marker and
	// can complete terminal status and cleanup without publishing a duplicate.
	s.crewRecoverInterruptedSession(s)
	if got := crewStepDesat(s, id); got != "done" {
		t.Fatalf("durable terminal status after recovery = %q", got)
	}
	if _, ok := loadCrewState(s, id); ok {
		t.Fatal("recovered terminal state was not cleaned up")
	}
	s.crewRecoverInterruptedSession(s)
	if got := crewResultCount(s, rot, result); got != 1 {
		t.Fatalf("result was republished during recovery: count=%d", got)
	}
}

func crewResultCount(s *Server, rot Rotllana, contains string) int {
	count := 0
	for _, message := range s.crewHistory(rot) {
		if message.Role == "assistant" && message.Kind == "result" && strings.Contains(message.Content, contains) {
			count++
		}
	}
	return count
}

func TestCrewProvenanceSurvivesStaleRoundAndQuestionSnapshots(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	st := &crewState{Task: "task", RunID: "same-run", Started: time.Now()}
	if err := saveCrewState(s, id, st); err != nil {
		t.Fatal(err)
	}
	// El fitxer compromès ha d'existir al workspace: noteCrewWriteDurable
	// valida l'escriptura real, no registra camins imaginaris.
	committed := filepath.Join(s.sessDir(), "project", "one.html")
	if err := os.MkdirAll(filepath.Dir(committed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(committed, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := noteCrewWriteDurable(s, id, s.sessDir(), committed); err != nil {
		t.Fatal(err)
	}
	// A round/question snapshot captured before the write must not erase it.
	st.Waiting = true
	if err := saveCrewState(s, id, st); err != nil {
		t.Fatal(err)
	}
	loaded, ok := loadCrewState(s, id)
	if !ok || len(loaded.Files) != 1 || loaded.Files[0] != "project/one.html" {
		t.Fatalf("write provenance lost at question checkpoint: %+v", loaded)
	}
	loaded.Waiting = false
	loaded.Questions++
	if err := saveCrewState(s, id, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, ok = loadCrewState(s, id)
	if !ok || len(loaded.Files) != 1 {
		t.Fatalf("write provenance lost after answer/resume checkpoint: %+v", loaded)
	}
}

func TestCrewRegistriesAreScopedToSessionOwner(t *testing.T) {
	s1, _ := rotllanaChatter(t, "same")
	s2, _ := rotllanaChatter(t, "same")
	s1.user = &User{Name: "owner-one"}
	s2.user = &User{Name: "owner-two"}
	const sameRotID = "same-rot"
	if crewRegistryKey(s1, sameRotID) == crewRegistryKey(s2, sameRotID) {
		t.Fatal("different owner session directories share a crew registry key")
	}
	noteCrewWrite(s1, sameRotID, "", "first.txt")
	if got := crewWriteSnapshot(s2, sameRotID); len(got) != 0 {
		t.Fatalf("write registry leaked across owners: %v", got)
	}
}

func TestCrewDistinctRunsMayPublishIdenticalResultText(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	rot, _ := s.rotllanaByID(id)
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		t.Fatal("missing lead wind")
	}
	for _, runID := range []string{"run-one", "run-two"} {
		if err := s.crewSayKindRun(rot, members[0], "result", "same result", runID); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, message := range s.crewHistory(rot) {
		if message.Kind == "result" && message.Content == members[0].Name+": same result" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("identical text from distinct run IDs was deduplicated: %d", count)
	}
}

func TestCrewRoundFlowMarksAssignedBuildersForPrivateWorkspaces(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	rot, _ := s.rotllanaByID(id)
	members := s.rotMembersOf(rot)
	if len(members) < 2 {
		t.Fatal("test crew needs builder and reviewer")
	}
	flow := crewRoundFlow([]crewAssignment{{Wind: members[0].ID, Part: "build the page", Files: []string{"index.html"}}}, members,
		func(companion.Wind, bool) string { return "Build or review." }, false)
	var builder, reviewer *team.FlowNode
	for i := range flow.Nodes {
		node := &flow.Nodes[i]
		if node.Role == "builder" {
			builder = node
		}
		if node.Role == "reviewer" {
			reviewer = node
		}
	}
	if builder == nil || !builder.CrewAssigned || !reflect.DeepEqual(builder.Files, []string{"index.html"}) {
		t.Fatalf("crew builder assignment marker/paths missing: %+v", builder)
	}
	if reviewer == nil || !reviewer.RequireStructuredReview {
		t.Fatalf("crew reviewer does not require a structured verdict: %+v", reviewer)
	}
	root := t.TempDir()
	workspaces, err := PrepareCrewAssignments(root, &flow)
	if err != nil {
		t.Fatal(err)
	}
	defer workspaces.Close()
	workspace, ok, err := workspaces.Workspace(builder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || workspace == root {
		t.Fatalf("crew builder did not receive a private project copy: %q", workspace)
	}
}
