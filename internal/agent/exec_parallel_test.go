package agent

import (
	"testing"
	"time"

	"gregal/internal/llm"
)

func mkCallPar(id, name, args string) llm.ToolCall {
	c := llm.ToolCall{ID: id, Type: "function"}
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

// L'allowlist només admet lectura pura: fitxers, cerques i xarxa de només
// lectura (web_search/fetch, read_image, office_read, gh_issue/pr).
// bash/edit/write/patch/todowrite/office_edit/create, bash_* i delegate mai.
func TestAllReadOnlyCalls(t *testing.T) {
	ro := []llm.ToolCall{
		mkCallPar("1", "read", "{}"),
		mkCallPar("2", "grep", "{}"),
	}
	if !allReadOnlyCalls(ro) {
		t.Fatalf("read+grep han de ser tot read-only")
	}
	ro2 := []llm.ToolCall{
		mkCallPar("1", "glob", "{}"),
		mkCallPar("2", "todoread", "{}"),
	}
	if !allReadOnlyCalls(ro2) {
		t.Fatalf("glob+todoread han de ser tot read-only")
	}
	ro3 := []llm.ToolCall{
		mkCallPar("1", "web_search", "{}"),
		mkCallPar("2", "web_fetch", "{}"),
		mkCallPar("3", "read_image", "{}"),
		mkCallPar("4", "office_read", "{}"),
		mkCallPar("5", "gh_issue", "{}"),
		mkCallPar("6", "gh_pr", "{}"),
	}
	if !allReadOnlyCalls(ro3) {
		t.Fatalf("les eines read-only de xarxa han de qualificar per paral·lel")
	}
	for _, name := range []string{"bash", "edit", "write", "patch", "todowrite", "bash_background", "delegate", "question", "office_edit", "office_create"} {
		mix := []llm.ToolCall{
			mkCallPar("1", "read", "{}"),
			mkCallPar("2", name, "{}"),
		}
		if allReadOnlyCalls(mix) {
			t.Fatalf("read+%s no pot ser tot read-only", name)
		}
	}
	if allReadOnlyCalls(nil) {
		t.Fatalf("bloc buit no pot ser tot read-only")
	}
}

// Dues lectures amb Exec fals de 200ms han de completar en < 1.8x el temps
// d'una sola (paral·lel) i mantenint l'ordre original.
func TestExecParallelReadOnly(t *testing.T) {
	fake := func(name, argsJSON string) (string, []string, error) {
		time.Sleep(200 * time.Millisecond)
		return "out:" + argsJSON, nil, nil
	}
	solo := []llm.ToolCall{mkCallPar("1", "read", `{"a":1}`)}
	t0 := time.Now()
	outs, _, errs := execParallel(solo, fake)
	tSolo := time.Since(t0)
	if len(outs) != 1 || errs[0] != nil || outs[0] != `out:{"a":1}` {
		t.Fatalf("solo: outs=%v errs=%v", outs, errs)
	}

	doble := []llm.ToolCall{
		mkCallPar("1", "read", `{"a":1}`),
		mkCallPar("2", "grep", `{"a":2}`),
	}
	if !allReadOnlyCalls(doble) {
		t.Fatalf("read+grep han de qualificar per paral·lel")
	}
	t1 := time.Now()
	outs2, _, errs2 := execParallel(doble, fake)
	tDoble := time.Since(t1)
	if len(outs2) != 2 || errs2[0] != nil || errs2[1] != nil {
		t.Fatalf("doble: outs=%v errs=%v", outs2, errs2)
	}
	if outs2[0] != `out:{"a":1}` || outs2[1] != `out:{"a":2}` {
		t.Fatalf("l'ordre original no es manté: %v", outs2)
	}
	limit := time.Duration(float64(tSolo) * 1.8)
	if tDoble >= limit {
		t.Fatalf("no és paral·lel: solo=%v doble=%v límit=%v", tSolo, tDoble, limit)
	}
}
