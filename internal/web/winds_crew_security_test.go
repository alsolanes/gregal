package web

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCrewCanceledClearsWritesAfterReporting(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	rot, _ := s.rotllanaByID(id)
	noteCrewWrite(id, "", "old-task.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.crewLoopContext(ctx, rot, &crewState{Task: "old task", Started: time.Now()}, &crewJob{})
	if writes := takeCrewWrites(id); len(writes) != 0 {
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
	crewJobs.Store(id, job)
	defer crewJobs.Delete(id)
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
			if s.crewTakeAnswer(rot, "answer") {
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
