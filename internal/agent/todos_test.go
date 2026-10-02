package agent

import (
	"context"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

func TestTodosDelPla(t *testing.T) {
	pla := `Pla:
1. Afegir el camp Timeout a config.go
2. **Propagar-lo** al client (internal/llm/client.go)
   - també al fallback
   3. això és una sub-llista, no un pas
3) Escriure el test a config_test.go
VERIFICACIÓ: go test ./internal/config/`
	items := TodosDelPla(pla)
	if len(items) != 3 {
		t.Fatalf("passos: %+v", items)
	}
	if items[0].Status != "working" || items[1].Status != "pending" || items[2].Status != "pending" {
		t.Fatalf("estats: %+v", items)
	}
	if items[1].Title != "Propagar-lo al client (internal/llm/client.go)" || !strings.HasPrefix(items[2].Title, "Escriure el test") {
		t.Fatalf("títols: %+v", items)
	}
	if TodosDelPla("Cap pas numerat aquí.\nNomés prosa.") != nil {
		t.Fatal("sense passos ha de tornar nil")
	}
}

func TestRecordatoriTodos(t *testing.T) {
	items := []tools.TodoItem{{Title: "Llegir", Status: "done"}, {Title: "Escriure el codi nou", Status: "working"}, {Title: "Tests", Status: "pending"}}
	if RecordatoriTodos(items, TodoNudgeEvery-1) != "" {
		t.Fatal("per sota del llindar no toca")
	}
	nota := RecordatoriTodos(items, TodoNudgeEvery)
	if !strings.Contains(nota, "1/3") || !strings.Contains(nota, "Escriure el codi nou") || !strings.Contains(nota, "todowrite") {
		t.Fatalf("nota: %q", nota)
	}
	tots := []tools.TodoItem{{Title: "A", Status: "done"}, {Title: "B", Status: "done"}}
	if RecordatoriTodos(tots, 20) != "" || RecordatoriTodos(nil, 20) != "" {
		t.Fatal("amb tot fet o sense checklist no toca")
	}
}

// El Loop de proves també executa en paral·lel i manté l'historial en ordre.
func TestLoopManteOrdreAmbParallel(t *testing.T) {
	step := 0
	l := &Loop{
		MaxSteps: 3,
		Step: func(_ context.Context, hist []llm.Message) (string, []llm.ToolCall, error) {
			step++
			if step == 1 {
				return "", []llm.ToolCall{crida("read", `{"path":"a"}`), crida("read", `{"path":"b"}`), crida("write", `{"path":"c"}`)}, nil
			}
			return "fet", nil, nil
		},
		RunTool: func(_ context.Context, name, args string) (string, []string, error) {
			return name + ":" + args, nil, nil
		},
		Decide: func(name, args string) (string, string) {
			if name == "write" {
				return "deny", "prova"
			}
			return "allow", ""
		},
	}
	_, hist, err := l.Run(context.Background(), "tasca")
	if err != nil {
		t.Fatal(err)
	}
	var sortides []string
	for _, m := range hist {
		if m.Role == "tool" {
			sortides = append(sortides, m.Content)
		}
	}
	if len(sortides) != 3 || sortides[0] != `read:{"path":"a"}` || sortides[1] != `read:{"path":"b"}` || !strings.HasPrefix(sortides[2], "EINA BLOQUEJADA") {
		t.Fatalf("historial: %v", sortides)
	}
}
