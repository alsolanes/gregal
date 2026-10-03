package team

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlanFeedsParallelIndependentBranchesAndReviewerMerge(t *testing.T) {
	var events []Event
	var callsMu sync.Mutex
	calls := make(map[string]int)
	var branchesStarted atomic.Int32
	branchesReady := make(chan struct{})

	answer, err := Run(context.Background(), "prepare report", func(ctx context.Context, role, task string) (string, error) {
		callsMu.Lock()
		calls[role]++
		callsMu.Unlock()

		switch role {
		case "coordinator":
			if !strings.Contains(task, "Task:\nprepare report") {
				t.Fatal("coordinator did not receive the task")
			}
			return "plan result", nil
		case "researcher", "builder":
			if !strings.Contains(task, "coordinator deliverable:\nplan result") || !strings.Contains(task, "Task:\nprepare report") {
				t.Errorf("%s did not receive the task and plan", role)
			}
			if role == "researcher" && strings.Contains(task, "builder result") {
				t.Error("researcher received the independent draft")
			}
			if role == "builder" && strings.Contains(task, "researcher result") {
				t.Error("builder received the independent analysis")
			}
			if branchesStarted.Add(1) == 2 {
				close(branchesReady)
			}
			select {
			case <-branchesReady:
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
				return "", errors.New("analysis and draft did not overlap")
			}
			return role + " result", nil
		case "reviewer":
			planAt := strings.Index(task, "coordinator deliverable:\nplan result")
			researchAt := strings.Index(task, "researcher deliverable:\nresearcher result")
			draftAt := strings.Index(task, "builder deliverable:\nbuilder result")
			if planAt < 0 || researchAt <= planAt || draftAt <= researchAt {
				t.Errorf("review context is incomplete or out of order: %s", task)
			}
			return "reviewer result", nil
		default:
			return "", errors.New("unexpected role: " + role)
		}
	}, func(event Event) { events = append(events, event) })
	if err != nil || answer != "reviewer result" {
		t.Fatalf("answer=%q err=%v", answer, err)
	}
	if calls["coordinator"] != 1 || calls["researcher"] != 1 || calls["builder"] != 1 || calls["reviewer"] != 1 {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	if got := eventCount(events, "discussion"); got != 0 {
		t.Fatalf("plain text review fabricated %d discussions", got)
	}
	if !hasActivity(events, "coordinator", "working", "plan") || !hasActivity(events, "researcher", "working", "analyze") || !hasActivity(events, "builder", "working", "draft") || !hasActivity(events, "reviewer", "completed", "review") {
		t.Fatalf("missing activity details: %#v", events)
	}
}

func TestRejectedReviewGetsOneRevisionAndOneFinalReview(t *testing.T) {
	var events []Event
	var callsMu sync.Mutex
	calls := make(map[string]int)
	var finalAnswer string

	answer, err := Run(context.Background(), "prepare report", func(ctx context.Context, role, prompt string) (string, error) {
		callsMu.Lock()
		calls[role]++
		roleCall := calls[role]
		callsMu.Unlock()

		switch {
		case role == "coordinator":
			return "plan result", nil
		case role == "researcher":
			return "analysis result", nil
		case role == "builder" && roleCall == 1:
			return "first draft", nil
		case role == "reviewer" && roleCall == 1:
			if !ordered(prompt, "coordinator deliverable:\nplan result", "researcher deliverable:\nanalysis result", "builder deliverable:\nfirst draft") {
				t.Errorf("first review did not receive ordered context: %s", prompt)
			}
			return `{"approved":false,"feedback":"add the missing risk","deliverable":"initial review"}`, nil
		case role == "builder" && roleCall == 2:
			if !strings.Contains(prompt, "builder deliverable:\nfirst draft") || !strings.Contains(prompt, "Feedback:\nadd the missing risk") || !strings.Contains(prompt, "Suggested deliverable:\ninitial review") {
				t.Errorf("revision did not receive its draft and feedback: %s", prompt)
			}
			if strings.Contains(prompt, "analysis result") {
				t.Error("revision received unrelated analysis")
			}
			return "revised draft", nil
		case role == "reviewer" && roleCall == 2:
			if !ordered(prompt, "coordinator deliverable:\nplan result", "researcher deliverable:\nanalysis result", "builder deliverable:\nrevised draft", "Previous review feedback:\nadd the missing risk") {
				t.Errorf("final review did not receive ordered context: %s", prompt)
			}
			finalAnswer = `{"approved":false,"feedback":"one risk still needs a source","deliverable":"final draft"}`
			return finalAnswer, nil
		default:
			return "", errors.New("unexpected role call")
		}
	}, func(event Event) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	want := "final draft\n\nOutstanding review feedback:\none risk still needs a source"
	if answer != want {
		t.Fatalf("answer=%q want=%q", answer, want)
	}
	if calls["coordinator"] != 1 || calls["researcher"] != 1 || calls["builder"] != 2 || calls["reviewer"] != 2 {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	var discussions []Event
	for _, event := range events {
		if event.Type == "discussion" {
			discussions = append(discussions, event)
		}
	}
	if len(discussions) != 1 || discussions[0].Agent != "reviewer" || discussions[0].To != "builder" || discussions[0].Activity != "revision" || discussions[0].Message != "add the missing risk" {
		t.Fatalf("unexpected public feedback: %#v", discussions)
	}
	if hasActivity(events, "builder", "working", "revision") == false {
		t.Fatalf("missing revision activity: %#v", events)
	}
	if !hasOutput(events, "reviewer", "final draft\n\nOutstanding review feedback:\none risk still needs a source") {
		t.Fatalf("final review output did not include outstanding feedback: %#v", events)
	}
}

func TestFailureCancelsParallelSiblingAndSkipsReview(t *testing.T) {
	researcherStarted := make(chan struct{})
	var siblingCanceled atomic.Bool
	var reviewerCalled atomic.Bool
	_, err := Run(context.Background(), "task", func(ctx context.Context, role, _ string) (string, error) {
		switch role {
		case "coordinator":
			return "plan", nil
		case "researcher":
			close(researcherStarted)
			<-ctx.Done()
			siblingCanceled.Store(true)
			return "", ctx.Err()
		case "builder":
			<-researcherStarted
			return "", errors.New("draft failed")
		case "reviewer":
			reviewerCalled.Store(true)
			return "unexpected", nil
		default:
			return "", errors.New("unexpected role")
		}
	}, func(Event) {})
	if err == nil || err.Error() != "draft failed" {
		t.Fatalf("err=%v", err)
	}
	if !siblingCanceled.Load() || reviewerCalled.Load() {
		t.Fatalf("siblingCanceled=%v reviewerCalled=%v", siblingCanceled.Load(), reviewerCalled.Load())
	}
}

func TestCoordinatorFailureStopsFollowingAgents(t *testing.T) {
	n := 0
	_, err := Run(context.Background(), "task", func(context.Context, string, string) (string, error) {
		n++
		return "", errors.New("failed")
	}, func(Event) {})
	if err == nil || n != 1 {
		t.Fatal("coordinator failure did not stop the run")
	}
}

func TestCancellationStopsFollowingAgents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	_, err := Run(ctx, "task", func(context.Context, string, string) (string, error) {
		n++
		cancel()
		return "result", nil
	}, func(Event) {})
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatalf("err=%v calls=%d", err, n)
	}
}

func TestParseReviewRequiresStrictStructuredFields(t *testing.T) {
	for _, output := range []string{
		`{"approved":false,"feedback":"  ","deliverable":"draft"}`,
		`{"approved":true,"feedback":"","deliverable":"draft","extra":true}`,
		`{"approved":false,"approved":true,"feedback":"","deliverable":"draft"}`,
		`{"approved":true,"deliverable":"draft"}`,
		`{"approved":true,"feedback":"","deliverable":"draft"} {}`,
		`{"approved":true,"feedback":"","deliverable":""}`,
	} {
		if _, err := parseReview(output); err == nil {
			t.Errorf("parseReview(%q) unexpectedly succeeded", output)
		}
	}
	if review, err := parseReview("plain text deliverable"); err != nil || review != nil {
		t.Fatalf("plain-text compatibility failed: review=%#v err=%v", review, err)
	}
}

func ordered(text string, parts ...string) bool {
	last := -1
	for _, part := range parts {
		index := strings.Index(text, part)
		if index <= last {
			return false
		}
		last = index
	}
	return true
}

func eventCount(events []Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func hasActivity(events []Event, agent, eventType, activity string) bool {
	for _, event := range events {
		if event.Agent == agent && event.Type == eventType && event.Activity == activity {
			return true
		}
	}
	return false
}

func hasOutput(events []Event, agent, output string) bool {
	for _, event := range events {
		if event.Agent == agent && event.Type == "completed" && event.Output == output {
			return true
		}
	}
	return false
}
