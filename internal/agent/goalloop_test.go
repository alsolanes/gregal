package agent

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"gregal/internal/goal"
)

const metReply = "VERDICT: MET\nCRITERIA:\n- [MET] file exists :: read a.txt, contains hello\nMISSING: none"
const notMetReply = "VERDICT: NOT MET\nCRITERIA:\n- [MET] file exists :: read a.txt\n- [NOT MET] tests added :: no test file in the diff\nMISSING: add a test"

func testGoal() goal.Goal {
	return goal.Goal{ID: "g1", Title: "write a.txt", Body: "task: write a.txt\ncriteria:\n- file exists\n- tests added"}
}

func fixedVerifier(replies ...string) (func(context.Context, string) (RunResult, error), *[]string) {
	var prompts []string
	i := 0
	return func(_ context.Context, p string) (RunResult, error) {
		prompts = append(prompts, p)
		r := replies[min(i, len(replies)-1)]
		i++
		return RunResult{Answer: r, Steps: 2}, nil
	}, &prompts
}

func TestGoalLoopStopsOnlyWhenVerifierConfirms(t *testing.T) {
	var tasks []string
	ver, prompts := fixedVerifier(notMetReply, metReply)
	round := 0
	res := RunGoalLoop(context.Background(), nil, nil, GoalLoopOpts{
		Goal: testGoal(), MaxRounds: 5,
		Run: RunOpts{Dir: t.TempDir(), MaxSteps: 10},
		runRound: func(_ context.Context, o RunOpts) (RunResult, error) {
			tasks = append(tasks, o.Task)
			round++
			return RunResult{Answer: "done round " + string(rune('0'+round)), Steps: 3}, nil
		},
		verifier: ver,
	})
	// Round 1 claims "done" but the verifier does not confirm: it must go on.
	if !res.Met || res.Stop != GoalMet || len(res.Rounds) != 2 {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(tasks[1], "NOT met") || !strings.Contains(tasks[1], "[NOT MET] tests added") || !strings.Contains(tasks[1], "add a test") {
		t.Fatalf("round 2 must carry the verifier's findings:\n%s", tasks[1])
	}
	if len(*prompts) != 2 || !strings.Contains((*prompts)[0], "THE AGENT'S OWN FINAL ANSWER (a claim, not evidence)") {
		t.Fatalf("verifier prompts: %v", *prompts)
	}
	if res.Steps != 3*2+2*2 || !strings.Contains(res.Summary, "GOAL MET after 2 round") {
		t.Fatalf("steps=%d summary=%s", res.Steps, res.Summary)
	}
}

func TestGoalLoopMetWithoutEvidenceIsNotMet(t *testing.T) {
	ver, _ := fixedVerifier("VERDICT: MET\nCRITERIA:\n- [MET] it works ::\nMISSING: none")
	res := RunGoalLoop(context.Background(), nil, nil, GoalLoopOpts{
		Goal: testGoal(), MaxRounds: 2, Run: RunOpts{Dir: t.TempDir()},
		runRound: func(_ context.Context, o RunOpts) (RunResult, error) {
			return RunResult{Answer: o.Task, Steps: 1}, nil
		},
		verifier: ver,
	})
	if res.Met || res.Stop == GoalMet {
		t.Fatalf("a MET without proof must not end the goal: %+v", res)
	}
}

func TestGoalLoopFailingCheckSkipsVerifierAndGuidesNextRound(t *testing.T) {
	failCmd, okCmd := "exit 7", "exit 0"
	if runtime.GOOS == "windows" {
		failCmd, okCmd = "exit /b 7", "exit /b 0"
	}
	ver, prompts := fixedVerifier(metReply)
	var tasks []string
	calls := 0
	checks := []string{failCmd}
	res := RunGoalLoop(context.Background(), nil, nil, GoalLoopOpts{
		Goal: testGoal(), Checks: checks, MaxRounds: 3, Run: RunOpts{Dir: t.TempDir()},
		runRound: func(_ context.Context, o RunOpts) (RunResult, error) {
			tasks = append(tasks, o.Task)
			calls++
			return RunResult{Answer: "r", Steps: 1}, nil
		},
		verifier: ver,
		OnRound: func(r GoalRound) {
			if r.N == 1 {
				checks[0] = okCmd // the agent "fixes" it for round 2 (shared slice)
			}
		},
	})
	if !res.Met || len(res.Rounds) != 2 {
		t.Fatalf("%+v", res)
	}
	if res.Rounds[0].Verified || len(*prompts) != 1 {
		t.Fatalf("a failed check must not spend a verifier call: %+v prompts=%d", res.Rounds[0], len(*prompts))
	}
	if !strings.Contains(tasks[1], "FAILED (exit code 7)") {
		t.Fatalf("failure output must guide round 2:\n%s", tasks[1])
	}
}

func TestGoalLoopLimits(t *testing.T) {
	notMet := func(i int) string {
		return strings.Replace(notMetReply, "add a test", "add test "+string(rune('a'+i)), 1)
	}
	base := func() GoalLoopOpts {
		n := 0
		return GoalLoopOpts{
			Goal: testGoal(), Run: RunOpts{Dir: t.TempDir(), MaxSteps: 50},
			runRound: func(_ context.Context, o RunOpts) (RunResult, error) {
				n++
				c := 0.5
				return RunResult{Answer: "a", Steps: 10, CostUSD: &c}, nil
			},
			verifier: func(ctx context.Context, p string) (RunResult, error) { return RunResult{Answer: notMet(0)}, nil },
		}
	}
	// Rounds. Identical evidence would stall first, so make the tree look different each time.
	o := base()
	cnt := 0
	o.runRound = func(_ context.Context, ro RunOpts) (RunResult, error) {
		cnt++
		return RunResult{Answer: strings.Repeat("x", cnt), Steps: 1}, nil
	}
	o.MaxRounds = 2
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopRounds || len(r.Rounds) != 2 || r.Met || !strings.Contains(r.Summary, "GOAL NOT MET") || !strings.Contains(r.Summary, "Still missing") {
		t.Fatalf("rounds: %+v\n%s", r, r.Summary)
	}
	// Steps.
	o = base()
	cnt = 0
	o.runRound = func(_ context.Context, ro RunOpts) (RunResult, error) {
		cnt++
		if ro.MaxSteps > 12 {
			t.Errorf("a round must not exceed the remaining step budget: %d", ro.MaxSteps)
		}
		return RunResult{Answer: strings.Repeat("y", cnt), Steps: 10}, nil
	}
	o.MaxSteps, o.MaxRounds = 12, 8
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopSteps {
		t.Fatalf("steps: %+v", r)
	}
	// Cost.
	o = base()
	cnt = 0
	o.runRound = func(_ context.Context, ro RunOpts) (RunResult, error) {
		cnt++
		c := 0.6
		return RunResult{Answer: strings.Repeat("z", cnt), Steps: 1, CostUSD: &c}, nil
	}
	o.verifier = func(ctx context.Context, p string) (RunResult, error) {
		c := 0.0
		return RunResult{Answer: notMet(0), CostUSD: &c}, nil
	}
	o.MaxCostUSD, o.MaxRounds = 1.0, 8
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopCost || r.CostUSD == nil || len(r.Rounds) != 2 {
		t.Fatalf("cost: %+v", r)
	}
	// Time: the round honours the deadline.
	o = base()
	o.runRound = func(ctx context.Context, ro RunOpts) (RunResult, error) {
		<-ctx.Done()
		return RunResult{Answer: "partial", Steps: 1}, nil
	}
	o.MaxMinutes = 0
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if r := RunGoalLoop(ctx, nil, nil, o); r.Stop != GoalStopCancelled && r.Stop != GoalStopTime {
		t.Fatalf("time: %+v", r)
	}
	// Stalled: identical evidence twice in a row.
	o = base()
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopStalled || len(r.Rounds) != 2 {
		t.Fatalf("stalled: %+v", r)
	}
}

func TestGoalLoopVerifierAndRunErrors(t *testing.T) {
	o := GoalLoopOpts{Goal: testGoal(), Run: RunOpts{Dir: t.TempDir()},
		runRound: func(context.Context, RunOpts) (RunResult, error) { return RunResult{Answer: "a", Steps: 1}, nil },
		verifier: func(context.Context, string) (RunResult, error) { return RunResult{}, errors.New("model down") },
	}
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopVerifier || r.Met {
		t.Fatalf("%+v", r)
	}
	o.runRound = func(context.Context, RunOpts) (RunResult, error) { return RunResult{}, errors.New("boom") }
	if r := RunGoalLoop(context.Background(), nil, nil, o); r.Stop != GoalStopError || !strings.Contains(r.Summary, "boom") {
		t.Fatalf("%+v", r)
	}
}
