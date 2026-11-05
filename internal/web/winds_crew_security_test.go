package web

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/team"
)

func TestCrewBuilderCannotWriteUnassignedFiles(t *testing.T) {
	if !teamFileAssignmentAllows("web/index.html", []string{"web/index.html"}) {
		t.Fatal("the explicitly assigned project file should be writable")
	}
	for _, path := range []string{"web/app.css", "../outside", "/etc/passwd", ""} {
		if teamFileAssignmentAllows(path, []string{"web/index.html"}) {
			t.Errorf("unassigned or unsafe path %q was authorized", path)
		}
	}
	if teamFileAssignmentAllows("web/index.html", nil) {
		t.Fatal("no assignment must authorize no file writes")
	}
	seat := seatFor(team.FlowNode{Role: "builder", CrewAssigned: true, Files: []string{"web/index.html"}}, nil, true)
	if !seat.EnforceFiles || len(seat.AllowedFiles) != 1 || seat.AllowedFiles[0] != "web/index.html" {
		t.Fatalf("flow assignment did not reach the builder seat: %+v", seat)
	}
	if !isTeamOtherWrite("office_edit") || isTeamFileWrite("office_edit") {
		t.Fatal("alternate office writer must be rejected rather than bypass file ownership")
	}
}

func TestCrewRoundVerdictDoesNotClaimUnanimity(t *testing.T) {
	v := crewRoundVerdict(func(c, _ string) string { return c }, "Review finished.", `{"approved":true}`, false)
	if strings.Contains(strings.ToLower(v), "agrees") || strings.Contains(strings.ToLower(v), "hi queda d'acord") {
		t.Fatalf("review verdict must not claim crew unanimity: %q", v)
	}
	if !strings.Contains(strings.ToLower(v), "revisió ha aprovat") {
		t.Fatalf("missing explicit reviewer approval: %q", v)
	}
}

func TestCrewCanceledClearsWritesAfterReporting(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	rot, _ := s.rotllanaByID(id)
	noteCrewWrite(s, id, "", "old-task.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.crewLoopContext(ctx, rot, &crewState{Task: "old task", Started: time.Now()}, &crewJob{})
	if writes := crewWriteSnapshot(s, id); len(writes) != 0 {
		t.Fatalf("cancel left stale writes: %v", writes)
	}
	found := false
	for _, m := range s.crewHistory(rot) {
		found = found || strings.Contains(m.Content, "old-task.txt")
	}
	if !found {
		t.Fatal("cancellation did not report the previously written file")
	}
}

func TestCrewResumeOnlyAcceptsOneConcurrentAnswer(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	rot, _ := s.rotllanaByID(id)
	st := &crewState{Task: "task", Waiting: true, Started: time.Now()}
	saveCrewState(s, id, st)
	job := &crewJob{path: crewJobPath(s, id)}
	job.set("waiting")
	key := crewRegistryKey(s, id)
	crewJobs.Store(key, job)
	defer crewJobs.Delete(key)
	old := crewCoordinator
	release := make(chan struct{})
	crewCoordinator = func(_ *Server, ctx context.Context, _, _ string) (string, error) {
		select {
		case <-release:
			return `{"action":"giveup","say":"done"}`, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	defer func() { close(release); waitBackground(); crewCoordinator = old }()
	var accepted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if s.crewTakeAnswer(rot, "answer") == crewAnswerConsumed {
				accepted.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted %d concurrent answers, want 1", got)
	}
}
