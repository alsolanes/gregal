// Package team coordinates bounded, tool-free collaboration between agents.
package team

import (
	"context"
	"errors"
	"strings"
)

type Event struct {
	Type   string `json:"type"`
	Agent  string `json:"agent,omitempty"`
	To     string `json:"to,omitempty"`
	Output string `json:"output,omitempty"`
}

type Runner func(context.Context, string, string) (string, error)

var Roles = []string{"coordinator", "researcher", "builder", "reviewer"}

// Run passes only public deliverables between roles, never private reasoning.
func Run(ctx context.Context, task string, runner Runner, emit func(Event)) (string, error) {
	instructions := []string{
		"Define a concise plan and acceptance criteria for this task.",
		"Analyze the provided information. Distinguish evidence, assumptions and missing information. Do not claim to browse or inspect files.",
		"Produce the requested deliverable using the plan and analysis. Do not claim to execute code or tools.",
		"Review the deliverable against the task and acceptance criteria. Return a corrected final deliverable and brief caveats.",
	}
	contextText := "Task:\n" + task
	var answer string
	for i, role := range Roles {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		emit(Event{Type: "working", Agent: role})
		out, err := runner(ctx, role, instructions[i]+"\n\n"+contextText)
		if err != nil {
			return "", err
		}
		out = strings.TrimSpace(out)
		if out == "" {
			return "", errors.New("empty agent deliverable")
		}
		// Bound both the displayed output and accumulated model context.
		runes := []rune(out)
		if len(runes) > 20000 {
			out = string(runes[:20000])
		}
		answer = out
		emit(Event{Type: "completed", Agent: role, Output: out})
		contextText += "\n\n" + role + " deliverable:\n" + out
		if i+1 < len(Roles) {
			emit(Event{Type: "handoff", Agent: role, To: Roles[i+1]})
		}
	}
	return answer, nil
}
