package agent

import (
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/llm"
)

func crida(name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID = name + args
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

// Quatre lectures seguides han d'anar alhora: si fossin seqüencials
// trigarien 4×80 ms; en paral·lel, ~80 ms.
func TestRunCallsLecturesEnParallel(t *testing.T) {
	calls := []llm.ToolCall{crida("read", "a"), crida("grep", "b"), crida("web_fetch", "c"), crida("glob", "d")}
	var alhora, maxAlhora int32
	t0 := time.Now()
	out := RunCalls(calls, nil, func(i int, c llm.ToolCall) string {
		n := atomic.AddInt32(&alhora, 1)
		for {
			m := atomic.LoadInt32(&maxAlhora)
			if n <= m || atomic.CompareAndSwapInt32(&maxAlhora, m, n) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		atomic.AddInt32(&alhora, -1)
		return c.Function.Arguments
	})
	if d := time.Since(t0); d > 250*time.Millisecond {
		t.Fatalf("no ha anat en paral·lel: %v", d)
	}
	if maxAlhora < 2 {
		t.Fatalf("màxim alhora %d", maxAlhora)
	}
	if out[0] != "a" || out[1] != "b" || out[2] != "c" || out[3] != "d" {
		t.Fatalf("ordre perdut: %v", out)
	}
}

// Les escriptures van d'una en una i en ordre, encara que hi hagi
// lectures entremig; el resultat sempre torna en l'ordre de les crides.
func TestRunCallsEscripturesEnOrdre(t *testing.T) {
	calls := []llm.ToolCall{crida("read", "1"), crida("write", "2"), crida("edit", "3"), crida("read", "4"), crida("read", "5"), crida("bash", "6")}
	var ordre []string
	var enCurs int32
	out := RunCalls(calls, nil, func(i int, c llm.ToolCall) string {
		if !ParallelSafe(c.Function.Name) {
			if atomic.AddInt32(&enCurs, 1) != 1 {
				t.Errorf("escriptura %s alhora amb una altra", c.Function.Arguments)
			}
			ordre = append(ordre, c.Function.Arguments)
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&enCurs, -1)
		}
		return c.Function.Arguments
	})
	if len(ordre) != 3 || ordre[0] != "2" || ordre[1] != "3" || ordre[2] != "6" {
		t.Fatalf("ordre d'escriptures: %v", ordre)
	}
	for i, c := range calls {
		if out[i] != c.Function.Arguments {
			t.Fatalf("resultat %d: %q", i, out[i])
		}
	}
}

// Les crides saltades (bloquejades, repetides) no s'executen i deixen el
// zero value; les lectures del voltant continuen agrupant-se.
func TestRunCallsSkip(t *testing.T) {
	calls := []llm.ToolCall{crida("read", "a"), crida("read", "b"), crida("read", "c")}
	var n int32
	out := RunCalls(calls, func(i int) bool { return i == 1 }, func(i int, c llm.ToolCall) string {
		atomic.AddInt32(&n, 1)
		return c.Function.Arguments
	})
	if n != 2 || out[1] != "" || out[0] != "a" || out[2] != "c" {
		t.Fatalf("skip: n=%d out=%v", n, out)
	}
}

func TestParallelSafe(t *testing.T) {
	for _, s := range []string{"read", "grep", "glob", "web_search", "web_fetch", "todoread", "delegate"} {
		if !ParallelSafe(s) {
			t.Errorf("%s hauria de ser segura", s)
		}
	}
	for _, s := range []string{"write", "edit", "patch", "bash", "bash_background", "todowrite", "office_edit", "mcp_x", "question"} {
		if ParallelSafe(s) {
			t.Errorf("%s no hauria de ser segura", s)
		}
	}
}
