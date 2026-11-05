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
	}, nil)
	if err == nil {
		t.Fatal("plain text review was accepted as approval")
	}
	if reviewCalls != 2 {
		t.Fatalf("review calls=%d, want retry then failure", reviewCalls)
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
