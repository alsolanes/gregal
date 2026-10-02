package web

import (
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

func srvTodo() *Server {
	return &Server{cfg: &config.Config{}, cwd: ".", mode: "code"}
}

func crida(nom, args string) llm.ToolCall {
	c := llm.ToolCall{ID: "t1"}
	c.Function.Name = nom
	c.Function.Arguments = args
	return c
}

// Amb dues pestanyes, la checklist d'una no pot ser la de l'altra: el
// magatzem de tools és un de sol per al procés i se les trepitjaven.
func TestChecklistPerSessio(t *testing.T) {
	a, b := srvTodo(), srvTodo()

	argsA, _ := json.Marshal(map[string]any{"items": []tools.TodoItem{
		{Title: "llegir el codi", Status: "done"},
		{Title: "arreglar el bug", Status: "working"},
	}})
	if out, _ := a.execTool(crida("todowrite", string(argsA))); !strings.Contains(out, "arreglar el bug") {
		t.Fatalf("la sessió A no ha desat la seva: %q", out)
	}

	argsB, _ := json.Marshal(map[string]any{"items": []tools.TodoItem{
		{Title: "escriure els tests", Status: "pending"},
	}})
	b.execTool(crida("todowrite", string(argsB)))

	// La de A ha de seguir sent la de A.
	outA, _ := a.execTool(crida("todoread", "{}"))
	if !strings.Contains(outA, "arreglar el bug") || strings.Contains(outA, "escriure els tests") {
		t.Fatalf("la sessió A veu la llista de la B: %q", outA)
	}
	outB, _ := b.execTool(crida("todoread", "{}"))
	if !strings.Contains(outB, "escriure els tests") || strings.Contains(outB, "arreglar el bug") {
		t.Fatalf("la sessió B veu la llista de l'A: %q", outB)
	}
	// I cap de les dues no ha tocat la global, que és la del TUI.
	if _, total := tools.TodoStats(); total != 0 {
		t.Fatalf("el web no ha de tocar la checklist global: %d ítems", total)
	}
}

// Un todowrite mal format no pot tombar el torn.
func TestChecklistArgsDolents(t *testing.T) {
	s := srvTodo()
	out, _ := s.execTool(crida("todowrite", "{no json"))
	if !strings.HasPrefix(out, "ERROR:") {
		t.Fatalf("hauria de tornar error: %q", out)
	}
}
