// Package team coordinates bounded, tool-free collaboration between agents.
package team

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Event struct {
	Type     string `json:"type"`
	Agent    string `json:"agent,omitempty"`
	To       string `json:"to,omitempty"`
	Output   string `json:"output,omitempty"`
	Activity string `json:"activity,omitempty"`
	Model    string `json:"model,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Runner func(context.Context, string, string) (string, error)

var Roles = []string{"coordinator", "researcher", "builder", "reviewer"}

const maxDeliverableRunes = 20000

type reviewResponse struct {
	Approved    *bool   `json:"approved"`
	Feedback    *string `json:"feedback"`
	Deliverable *string `json:"deliverable"`
}

// Run passes only public deliverables between roles, never private reasoning.
func Run(ctx context.Context, task string, runner Runner, emit func(Event)) (string, error) {
	if runner == nil {
		return "", errors.New("nil agent runner")
	}

	var emitMu sync.Mutex
	emitEvent := func(event Event) {
		if emit == nil {
			return
		}
		emitMu.Lock()
		defer emitMu.Unlock()
		emit(event)
	}

	task = strings.TrimSpace(task)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	plan, err := runRole(ctx, runner, emitEvent, "coordinator", "plan", "Define a concise plan and acceptance criteria for this task.", "Task:\n"+task)
	if err != nil {
		return "", err
	}

	emitEvent(Event{Type: "handoff", Agent: "coordinator", To: "researcher", Activity: "analyze"})
	emitEvent(Event{Type: "handoff", Agent: "coordinator", To: "builder", Activity: "draft"})
	emitEvent(Event{Type: "working", Agent: "researcher", Activity: "analyze"})
	emitEvent(Event{Type: "working", Agent: "builder", Activity: "draft"})

	branchContext, cancelBranches := context.WithCancel(ctx)
	defer cancelBranches()
	branchResults := make(chan roleResult, 2)
	for _, branch := range []struct {
		role        string
		instruction string
	}{
		{"researcher", "Analyze the provided information. Distinguish evidence, assumptions and missing information. Do not claim to browse or inspect files."},
		{"builder", "Produce the requested deliverable using the plan. Do not claim to execute code or tools."},
	} {
		branch := branch
		prompt := withDeliverables(task, []roleDeliverable{{role: "coordinator", output: plan}})
		go func() {
			out, err := callRunner(branchContext, runner, branch.role, branch.instruction, prompt)
			branchResults <- roleResult{role: branch.role, output: out, err: err}
		}()
	}

	parallel := make(map[string]roleResult, 2)
	for len(parallel) < 2 {
		select {
		case result := <-branchResults:
			parallel[result.role] = result
			if result.err == nil {
				emitEvent(Event{Type: "completed", Agent: result.role, Activity: activityFor(result.role), Output: result.output})
			}
			if result.err != nil {
				cancelBranches()
			}
		case <-ctx.Done():
			cancelBranches()
			return "", ctx.Err()
		}
	}
	if err := parallelError(parallel); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	research := parallel["researcher"].output
	draft := parallel["builder"].output
	emitEvent(Event{Type: "handoff", Agent: "researcher", To: "reviewer", Activity: "review"})
	emitEvent(Event{Type: "handoff", Agent: "builder", To: "reviewer", Activity: "review"})

	reviewInput := withDeliverables(task, []roleDeliverable{
		{role: "coordinator", output: plan},
		{role: "researcher", output: research},
		{role: "builder", output: draft},
	})
	firstReview, err := reviewRole(ctx, runner, emitEvent, reviewInput, false)
	if err != nil {
		return "", err
	}
	if firstReview.approved {
		return firstReview.deliverable, nil
	}

	emitEvent(Event{
		Type:     "discussion",
		Agent:    "reviewer",
		To:       "builder",
		Activity: "revision",
		Message:  firstReview.feedback,
	})
	emitEvent(Event{Type: "working", Agent: "builder", Activity: "revision"})
	revisionPrompt := withDeliverables(task, []roleDeliverable{
		{role: "coordinator", output: plan},
		{role: "builder", output: draft},
		{role: "reviewer", output: "Feedback:\n" + firstReview.feedback + "\n\nSuggested deliverable:\n" + firstReview.deliverable},
	})
	revisedDraft, err := callRunner(ctx, runner, "builder", "Revise the draft using the reviewer's feedback. Do not claim to execute code or tools.", revisionPrompt)
	if err != nil {
		return "", err
	}
	emitEvent(Event{Type: "completed", Agent: "builder", Activity: "revision", Output: revisedDraft})
	emitEvent(Event{Type: "handoff", Agent: "builder", To: "reviewer", Activity: "review"})

	finalReviewInput := withDeliverables(task, []roleDeliverable{
		{role: "coordinator", output: plan},
		{role: "researcher", output: research},
		{role: "builder", output: revisedDraft},
		{role: "reviewer", output: "Previous review feedback:\n" + firstReview.feedback},
	})
	finalReview, err := reviewRole(ctx, runner, emitEvent, finalReviewInput, true)
	if err != nil {
		return "", err
	}
	return finalReview.deliverable, nil
}

type roleResult struct {
	role   string
	output string
	err    error
}

type roleDeliverable struct {
	role   string
	output string
}

type parsedReview struct {
	approved    bool
	feedback    string
	deliverable string
}

func runRole(ctx context.Context, runner Runner, emit func(Event), role, activity, instruction, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	emit(Event{Type: "working", Agent: role, Activity: activity})
	out, err := callRunner(ctx, runner, role, instruction, prompt)
	if err != nil {
		return "", err
	}
	emit(Event{Type: "completed", Agent: role, Activity: activity, Output: out})
	return out, nil
}

func callRunner(ctx context.Context, runner Runner, role, instruction, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	out, err := runner(ctx, role, instruction+"\n\n"+prompt)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", errors.New("empty agent deliverable")
	}
	runes := []rune(out)
	if len(runes) > maxDeliverableRunes {
		out = string(runes[:maxDeliverableRunes])
	}
	return out, nil
}

func reviewRole(ctx context.Context, runner Runner, emit func(Event), prompt string, final bool) (*parsedReview, error) {
	instruction := `Review the task and deliverable against the plan and acceptance criteria. Return exactly one JSON object with all three fields: {"approved": boolean, "feedback": string, "deliverable": string}. If approved is false, feedback must give actionable revision guidance. deliverable must contain the corrected final deliverable. Do not claim to execute code or tools.`
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	emit(Event{Type: "working", Agent: "reviewer", Activity: "review"})
	output, err := callRunner(ctx, runner, "reviewer", instruction, prompt)
	if err != nil {
		return nil, err
	}
	review, err := parseReview(output)
	if err != nil {
		return nil, err
	}
	if review == nil {
		// Plain-text reviewer responses remain supported for older providers.
		review = &parsedReview{approved: true, deliverable: output}
	}
	if final && !review.approved {
		review.deliverable += "\n\nOutstanding review feedback:\n" + review.feedback
	}
	emit(Event{Type: "completed", Agent: "reviewer", Activity: "review", Output: review.deliverable})
	return review, nil
}

func parseReview(output string) (*parsedReview, error) {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") {
		// Plain-text reviewer responses remain supported for older providers.
		return nil, nil
	}
	if err := rejectDuplicateFields(trimmed); err != nil {
		return nil, fmt.Errorf("invalid structured reviewer response: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var response reviewResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("invalid structured reviewer response: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("invalid structured reviewer response: multiple JSON values")
		}
		return nil, fmt.Errorf("invalid structured reviewer response: %w", err)
	}
	if response.Approved == nil || response.Feedback == nil || response.Deliverable == nil {
		return nil, errors.New("invalid structured reviewer response: all fields are required")
	}
	feedback := strings.TrimSpace(*response.Feedback)
	deliverable := strings.TrimSpace(*response.Deliverable)
	if deliverable == "" {
		return nil, errors.New("invalid structured reviewer response: empty deliverable")
	}
	if !*response.Approved && feedback == "" {
		return nil, errors.New("invalid structured reviewer response: rejected review requires feedback")
	}
	return &parsedReview{approved: *response.Approved, feedback: feedback, deliverable: deliverable}, nil
}

func rejectDuplicateFields(input string) error {
	decoder := json.NewDecoder(strings.NewReader(input))
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return errors.New("review must be a JSON object")
	}
	seen := make(map[string]bool, 3)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("review object contains an invalid field")
		}
		if seen[key] {
			return fmt.Errorf("duplicate review field %q", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := closing.(json.Delim); !ok || delimiter != '}' {
		return errors.New("review must be a JSON object")
	}
	return nil
}

func parallelError(results map[string]roleResult) error {
	for _, role := range []string{"researcher", "builder"} {
		if err := results[role].err; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	for _, role := range []string{"researcher", "builder"} {
		if err := results[role].err; err != nil {
			return err
		}
	}
	return nil
}

func activityFor(role string) string {
	switch role {
	case "coordinator":
		return "plan"
	case "researcher":
		return "analyze"
	case "builder":
		return "draft"
	case "reviewer":
		return "review"
	default:
		return ""
	}
}

func withDeliverables(task string, deliverables []roleDeliverable) string {
	var contextText strings.Builder
	contextText.WriteString("Task:\n")
	contextText.WriteString(task)
	for _, deliverable := range deliverables {
		contextText.WriteString("\n\n")
		contextText.WriteString(deliverable.role)
		contextText.WriteString(" deliverable:\n")
		contextText.WriteString(deliverable.output)
	}
	return contextText.String()
}
