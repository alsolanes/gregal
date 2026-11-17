package agent

import (
	"testing"

	"gregal/internal/llm"
)

func TestExecParallelTurnsPanicIntoError(t *testing.T) {
	calls := []llm.ToolCall{{}, {}}
	calls[0].Function.Name, calls[1].Function.Name = "read", "grep"
	outs, _, errs := execParallel(calls, func(name, _ string) (string, []string, error) {
		if name == "read" {
			panic("boom")
		}
		return "ok", nil, nil
	})
	if errs[0] == nil || outs[1] != "ok" || errs[1] != nil {
		t.Fatalf("outs=%v errs=%v", outs, errs)
	}
}
