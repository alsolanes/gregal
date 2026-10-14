package team

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// FlowNode is one agent seat in a custom pipeline. ID is unique within the
// spec and is what events and deliverables refer to; Role picks the
// character (coordinator|researcher|builder|reviewer) and the default
// instruction. Mode is "task" (produce a deliverable, default) or "review"
// (strict approved/feedback/deliverable verdict).
type FlowNode struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Label       string `json:"label,omitempty"`
	Instruction string `json:"instruction,omitempty"`
	Mode        string `json:"mode,omitempty"`
	// Agent referencia un subagent definit per l'usuari (.gregal/agents):
	// el runner hi fa servir les seves eines, el seu model i el seu prompt.
	// El paquet team no el valida (no sap on és el projecte): ho fa qui
	// executa el flux.
	Agent string `json:"agent,omitempty"`
}

// FlowEdge carries a deliverable (data) or asks for rework (revision).
// Revision edges may only start at a review node and must point at a node
// that feeds that review; otherwise the reviewer would re-run on stale
// inputs and the flow could stall.
type FlowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// FlowSpec is a team pipeline. Output is the node ID whose deliverable is
// the answer. MaxRounds bounds each revision edge (0 = review as gate:
// reject finalizes without rework).
type FlowSpec struct {
	Name      string     `json:"name,omitempty"`
	Nodes     []FlowNode `json:"nodes"`
	Edges     []FlowEdge `json:"edges"`
	Output    string     `json:"output"`
	MaxRounds int        `json:"max_rounds,omitempty"`
}

const (
	flowModeTask     = "task"
	flowModeReview   = "review"
	flowKindData     = "data"
	flowKindRevision = "revision"
)

// DefaultFlow is today's fixed pipeline as data: plan, parallel
// investigate/draft, review, one revision round back to the builder.
func DefaultFlow() FlowSpec {
	return FlowSpec{
		Name: "default",
		Nodes: []FlowNode{
			{ID: "coordinator", Role: "coordinator", Mode: flowModeTask, Instruction: "Define a concise plan and acceptance criteria for this task."},
			{ID: "researcher", Role: "researcher", Mode: flowModeTask, Instruction: "Analyze the provided information. Distinguish evidence, assumptions and missing information. Never claim to have browsed or inspected anything you did not actually inspect."},
			{ID: "builder", Role: "builder", Mode: flowModeTask, Instruction: "Produce the requested deliverable using the plan. Never claim to have run code or tools you did not actually run."},
			{ID: "reviewer", Role: "reviewer", Mode: flowModeReview},
		},
		Edges: []FlowEdge{
			{From: "coordinator", To: "researcher", Kind: flowKindData},
			{From: "coordinator", To: "builder", Kind: flowKindData},
			{From: "researcher", To: "reviewer", Kind: flowKindData},
			{From: "builder", To: "reviewer", Kind: flowKindData},
			{From: "reviewer", To: "builder", Kind: flowKindRevision},
		},
		Output:    "reviewer",
		MaxRounds: 1,
	}
}

// ValidateFlow rejects unusable pipelines before any model call is spent.
func ValidateFlow(spec FlowSpec) error {
	if len(spec.Nodes) == 0 {
		return errors.New("flow needs at least one node")
	}
	byID := make(map[string]FlowNode, len(spec.Nodes))
	for _, node := range spec.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return errors.New("flow node needs an id")
		}
		if _, dup := byID[node.ID]; dup {
			return fmt.Errorf("duplicate flow node %q", node.ID)
		}
		knownRole := false
		for _, role := range Roles {
			if node.Role == role {
				knownRole = true
				break
			}
		}
		if !knownRole {
			return fmt.Errorf("unknown flow role %q (coordinator|researcher|builder|reviewer)", node.Role)
		}
		if node.Mode != "" && node.Mode != flowModeTask && node.Mode != flowModeReview {
			return fmt.Errorf("flow node %q has invalid mode %q (task|review)", node.ID, node.Mode)
		}
		if node.Instruction == "" && !hasDefaultInstruction(node) {
			return fmt.Errorf("flow node %q (%s/%s) needs an explicit instruction", node.ID, node.Role, modeOf(node))
		}
		byID[node.ID] = node
	}
	if _, ok := byID[spec.Output]; !ok {
		return fmt.Errorf("flow output %q is not a node", spec.Output)
	}
	if spec.MaxRounds < 0 {
		return errors.New("flow max_rounds cannot be negative")
	}
	downstream := map[string][]string{}
	for _, edge := range spec.Edges {
		from, okFrom := byID[edge.From]
		if _, okTo := byID[edge.To]; !okFrom || !okTo {
			return fmt.Errorf("flow edge %q>%q refers to an unknown node", edge.From, edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("flow edge %q>%q is a self loop", edge.From, edge.To)
		}
		if edge.Kind != flowKindData && edge.Kind != flowKindRevision {
			return fmt.Errorf("flow edge %q>%q has invalid kind %q (data|revision)", edge.From, edge.To, edge.Kind)
		}
		if edge.Kind == flowKindRevision {
			if modeOf(from) != flowModeReview {
				return fmt.Errorf("revision edge %q>%q must start at a review node", edge.From, edge.To)
			}
			if !reaches(edge.To, edge.From, downstreamData(spec)) {
				return fmt.Errorf("revision edge %q>%q must point at work that feeds the review", edge.From, edge.To)
			}
			continue
		}
		downstream[edge.From] = append(downstream[edge.From], edge.To)
	}
	if hasDataCycle(spec) {
		return errors.New("flow data edges contain a cycle")
	}
	for _, node := range spec.Nodes {
		if !reaches(node.ID, spec.Output, downstream) {
			return fmt.Errorf("flow node %q never reaches the output", node.ID)
		}
	}
	if len(downstream[spec.Output]) > 0 {
		return fmt.Errorf("flow output %q must not feed other nodes", spec.Output)
	}
	return nil
}

func modeOf(node FlowNode) string {
	if node.Mode == "" {
		return flowModeTask
	}
	return node.Mode
}

// hasDefaultInstruction is true for the four classic role/mode pairs; any
// other combination must spell out its instruction.
func hasDefaultInstruction(node FlowNode) bool {
	mode := modeOf(node)
	return (node.Role == "coordinator" && mode == flowModeTask) ||
		(node.Role == "researcher" && mode == flowModeTask) ||
		(node.Role == "builder" && mode == flowModeTask) ||
		(node.Role == "reviewer" && mode == flowModeReview)
}

func downstreamData(spec FlowSpec) map[string][]string {
	downstream := map[string][]string{}
	for _, edge := range spec.Edges {
		if edge.Kind == flowKindData {
			downstream[edge.From] = append(downstream[edge.From], edge.To)
		}
	}
	return downstream
}

func reaches(from, to string, downstream map[string][]string) bool {
	if from == to {
		return true
	}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range downstream[id] {
			if next == to {
				return true
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

func hasDataCycle(spec FlowSpec) bool {
	indegree := map[string]int{}
	downstream := downstreamData(spec)
	for _, node := range spec.Nodes {
		if _, ok := indegree[node.ID]; !ok {
			indegree[node.ID] = 0
		}
		for _, to := range downstream[node.ID] {
			indegree[to]++
		}
	}
	var queue []string
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range downstream[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return visited != len(spec.Nodes)
}

type flowResult struct {
	id      string
	output  string
	verdict *parsedReview
	err     error
}

// revRequest is a pending rework order: the target re-runs with its prior
// draft plus the reviewer feedback, mirroring the classic revision round.
type revRequest struct {
	prior     string
	feedback  string
	suggested string
	reviewer  string
}

// RunFlow executes a validated custom pipeline with the same event
// vocabulary as Run (working/completed/handoff/discussion/done), so the
// room, the graph and the activity feed work unchanged. Ready nodes run in
// parallel batches; the first hard error cancels its siblings.
func RunFlow(ctx context.Context, task string, spec FlowSpec, runner Runner, emit func(Event)) (string, error) {
	if runner == nil {
		return "", errors.New("nil agent runner")
	}
	if err := ValidateFlow(spec); err != nil {
		return "", err
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
	byID := make(map[string]FlowNode, len(spec.Nodes))
	for _, node := range spec.Nodes {
		byID[node.ID] = node
	}
	labelOf := func(id string) string {
		if label := strings.TrimSpace(byID[id].Label); label != "" {
			return label
		}
		return byID[id].Role
	}
	outputs := map[string]string{}
	done := map[string]bool{}
	revUsed := map[string]int{}
	revPending := map[string]revRequest{}
	promptFor := func(id string) string {
		var deliverables []roleDeliverable
		for _, node := range spec.Nodes {
			output, ok := outputs[node.ID]
			if !ok || !isDataDep(node.ID, id, spec) {
				continue
			}
			deliverables = append(deliverables, roleDeliverable{role: labelOf(node.ID), output: output})
		}
		if req, ok := revPending[id]; ok {
			if req.prior != "" {
				deliverables = append(deliverables, roleDeliverable{role: labelOf(id), output: req.prior})
			}
			block := "Feedback:\n" + req.feedback
			if strings.TrimSpace(req.suggested) != "" {
				block += "\n\nSuggested deliverable:\n" + req.suggested
			}
			deliverables = append(deliverables, roleDeliverable{role: req.reviewer, output: block})
		}
		return withDeliverables(task, deliverables)
	}

	branchContext, cancelBranches := context.WithCancel(ctx)
	defer cancelBranches()

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		var batch []string
		for _, node := range spec.Nodes {
			if done[node.ID] {
				continue
			}
			ready := true
			for _, edge := range spec.Edges {
				if edge.Kind == flowKindData && edge.To == node.ID && !done[edge.From] {
					ready = false
					break
				}
			}
			if ready {
				batch = append(batch, node.ID)
			}
		}
		if len(batch) == 0 {
			return "", errors.New("flow stalled with no runnable node")
		}
		results := make(chan flowResult, len(batch))
		for _, id := range batch {
			id := id
			go func() {
				node := byID[id]
				nodeCtx := WithNode(branchContext, node)
				emitEvent(Event{Type: "working", Agent: id, Activity: activityFor(node.Role)})
				if modeOf(node) == flowModeReview {
					verdict, err := judgeNode(nodeCtx, runner, reviewInstruction(node), promptFor(id))
					results <- flowResult{id: id, verdict: verdict, err: err}
					return
				}
				out, err := callRunner(nodeCtx, runner, id, instructionOf(node), promptFor(id))
				results <- flowResult{id: id, output: out, err: err}
			}()
		}
		byResult := make(map[string]flowResult, len(batch))
		for range batch {
			select {
			case result := <-results:
				byResult[result.id] = result
				if result.err != nil {
					cancelBranches()
				}
			case <-ctx.Done():
				cancelBranches()
				return "", ctx.Err()
			}
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := flowBatchError(byResult); err != nil {
			return "", err
		}
		for _, id := range batch {
			result := byResult[id]
			node := byID[id]
			if modeOf(node) == flowModeReview {
				verdict := result.verdict
				if verdict.approved {
					outputs[id] = verdict.deliverable
					done[id] = true
					emitEvent(Event{Type: "completed", Agent: id, Activity: activityFor(node.Role), Output: verdict.deliverable})
					emitHandoffs(emitEvent, spec, byID, id)
					continue
				}
				fired := false
				for _, edge := range spec.Edges {
					if edge.Kind != flowKindRevision || edge.From != id {
						continue
					}
					key := edge.From + ">" + edge.To
					if revUsed[key] >= spec.MaxRounds {
						continue
					}
					revUsed[key]++
					fired = true
					revPending[edge.To] = revRequest{
						prior:     outputs[edge.To],
						feedback:  verdict.feedback,
						suggested: verdict.deliverable,
						reviewer:  labelOf(id),
					}
					for stale := range downstreamClosure(edge.To, spec) {
						delete(done, stale)
						delete(outputs, stale)
					}
					emitEvent(Event{Type: "completed", Agent: id, Activity: activityFor(node.Role), Output: verdict.deliverable})
					emitEvent(Event{Type: "discussion", Agent: id, To: edge.To, Activity: "revision", Message: verdict.feedback})
				}
				if !fired {
					deliverable := verdict.deliverable + "\n\nOutstanding review feedback:\n" + verdict.feedback
					outputs[id] = deliverable
					done[id] = true
					delete(revPending, id)
					emitEvent(Event{Type: "completed", Agent: id, Activity: activityFor(node.Role), Output: deliverable})
					emitHandoffs(emitEvent, spec, byID, id)
				}
				continue
			}
			outputs[id] = result.output
			done[id] = true
			delete(revPending, id)
			emitEvent(Event{Type: "completed", Agent: id, Activity: activityFor(node.Role), Output: result.output})
			emitHandoffs(emitEvent, spec, byID, id)
		}
		if done[spec.Output] {
			return outputs[spec.Output], nil
		}
	}
}

func emitHandoffs(emitEvent func(Event), spec FlowSpec, byID map[string]FlowNode, id string) {
	for _, edge := range spec.Edges {
		if edge.Kind == flowKindData && edge.From == id {
			emitEvent(Event{Type: "handoff", Agent: id, To: edge.To, Activity: activityFor(byID[edge.To].Role)})
		}
	}
}

// isDataDep reports whether from feeds into into over data edges.
func isDataDep(from, into string, spec FlowSpec) bool {
	for _, edge := range spec.Edges {
		if edge.Kind == flowKindData && edge.From == from && edge.To == into {
			return true
		}
	}
	return false
}

// downstreamClosure returns id plus everything reachable from it over data
// edges, for invalidating stale work on revision.
func downstreamClosure(id string, spec FlowSpec) map[string]bool {
	closure := map[string]bool{id: true}
	queue := []string{id}
	downstream := downstreamData(spec)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range downstream[current] {
			if !closure[next] {
				closure[next] = true
				queue = append(queue, next)
			}
		}
	}
	return closure
}

func flowBatchError(results map[string]flowResult) error {
	for _, result := range results {
		if err := result.err; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	for _, result := range results {
		if result.err != nil {
			return result.err
		}
	}
	return nil
}

// instructionOf returns the custom instruction or the classic default for
// the four classic role/mode pairs (validation guarantees one exists).
func instructionOf(node FlowNode) string {
	if strings.TrimSpace(node.Instruction) != "" {
		return node.Instruction
	}
	switch {
	case node.Role == "coordinator" && modeOf(node) == flowModeTask:
		return "Define a concise plan and acceptance criteria for this task."
	case node.Role == "researcher" && modeOf(node) == flowModeTask:
		return "Analyze the provided information. Distinguish evidence, assumptions and missing information. Never claim to have browsed or inspected anything you did not actually inspect."
	case node.Role == "builder" && modeOf(node) == flowModeTask:
		return "Produce the requested deliverable using the plan. Never claim to have run code or tools you did not actually run."
	default:
		return ""
	}
}

// reviewInstruction returns the custom rubric or the classic strict one.
func reviewInstruction(node FlowNode) string {
	if strings.TrimSpace(node.Instruction) != "" {
		return node.Instruction
	}
	return `Review the task and deliverable against the plan and acceptance criteria. Return exactly one JSON object with all three fields: {"approved": boolean, "feedback": string, "deliverable": string}. If approved is false, feedback must give actionable revision guidance. deliverable must contain the corrected final deliverable. Never claim to have run checks you did not actually run.`
}

// judgeNode runs one review verdict without emitting: the engine decides
// what the verdict means (rework, finalize) and emits accordingly.
func judgeNode(ctx context.Context, runner Runner, instruction, prompt string) (*parsedReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, err := callRunner(ctx, runner, "reviewer", instruction, prompt)
	if err != nil {
		return nil, err
	}
	review, err := parseReview(output)
	if err != nil {
		return nil, err
	}
	if review == nil {
		review = &parsedReview{approved: true, deliverable: output}
	}
	return review, nil
}
