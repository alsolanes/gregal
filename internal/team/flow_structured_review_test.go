package team

import (
	"context"
	"strings"
	"testing"
)

func TestCrewStructuredReviewRejectsPlainTextAndToolslessVerdict(t *testing.T) {
	spec := FlowSpec{
		Nodes: []FlowNode{
			{ID: "build", Role: "builder", Instruction: "build"},
			{ID: "review", Role: "reviewer", Mode: flowModeReview, RequireStructuredReview: true},
		},
		Edges:  []FlowEdge{{From: "build", To: "review", Kind: flowKindData}},
		Output: "review",
	}
	var reviewCalls int
	invalid := false
	_, err := RunFlow(context.Background(), "task", spec, func(ctx context.Context, id, prompt string) (string, error) {
		switch id {
		case "build":
			return "artifact", nil
		case "review":
			reviewCalls++
			if strings.Contains(prompt, "previous answer could not be used") {
				return "I approve the work.", nil
			}
			return "Looks good; approved.", nil
		default:
			t.Fatalf("unexpected node %s", id)
			return "", nil
		}
	}, func(e Event) {
		if e.Type == "completed" && e.Agent == "review" && e.ReviewInvalid {
			invalid = true
		}
	})
	// Ni aprovació ni feina perduda: la feina es queda, marcada com a no
	// verificada (la colla ho diu clar a l'informe).
	if err != nil {
		t.Fatalf("a botched verdict must not throw away the work: %v", err)
	}
	if !invalid {
		t.Fatal("plain text review was accepted as a verified approval")
	}
	if reviewCalls != 2 {
		t.Fatalf("review calls=%d, want one retry", reviewCalls)
	}
}

func TestGenericFlowStillAcceptsLegacyPlainTextReview(t *testing.T) {
	spec := FlowSpec{
		Nodes: []FlowNode{{ID: "build", Role: "builder", Instruction: "build"}, {ID: "review", Role: "reviewer", Mode: flowModeReview}},
		Edges: []FlowEdge{{From: "build", To: "review", Kind: flowKindData}}, Output: "review",
	}
	out, err := RunFlow(context.Background(), "task", spec, func(ctx context.Context, id, prompt string) (string, error) {
		if id == "build" {
			return "artifact", nil
		}
		return "looks good", nil
	}, nil)
	if err != nil || !strings.Contains(out, "looks good") {
		t.Fatalf("generic legacy verdict result=%q err=%v", out, err)
	}
}

func TestStructuredReviewAcceptsJSONAfterProseAndMissingDeliverable(t *testing.T) {
	r, err := parseReview("Ho he revisat tot.\n\n{\"approved\": false, \"feedback\": \"falta el test\"}")
	if err != nil || r == nil || r.approved || r.feedback != "falta el test" || r.deliverable != "" {
		t.Fatalf("%+v %v", r, err)
	}
}
