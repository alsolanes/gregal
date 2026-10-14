package web

import (
	"errors"
	"strings"
	"testing"
)

func TestRetroDiagnosticSenseModel(t *testing.T) {
	c := &retroChannel{Lang: "en", Agents: []retroAgent{{ID: "coordinator", Name: "Coordinator"}},
		Evidence: []retroEvidence{{ID: "e1", Agent: "coordinator", Title: `pas 1: provider https://opencode.ai/zen/go/v1: HTTP 401: {"type":"error"}`}}}
	msgs := retroFallbackStandup(c, errors.New(`provider [url] HTTP 400: {"error":{"message":"Upstream request failed: Model is unavailable."}}`))
	if len(msgs) != 1 {
		t.Fatalf("msgs=%d", len(msgs))
	}
	txt := msgs[0].Text
	if strings.Contains(txt, "https://") || strings.Contains(txt, `{"`) {
		t.Fatalf("raw URL/JSON leaked: %q", txt)
	}
	if !strings.Contains(txt, "API key") {
		t.Fatalf("expected an actionable diagnosis, got %q", txt)
	}
	if d := retroDiagnose(&retroChannel{Lang: "ca"}, errors.New("Model is unavailable")); !strings.Contains(d, "no està disponible") {
		t.Fatalf("catalan diagnosis: %q", d)
	}
}
