package tui

import (
	"strings"
	"testing"

	"gregal/internal/tools"
)

// Executar un pla aprovat ha de publicar els passos numerats al checklist
// abans que el model faci res, amb el primer en marxa i el cockpit obert.
func TestExecutarPlaSembraElChecklist(t *testing.T) {
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	m := planTestModel(t)
	m.pendingPlan = "1. Afegir el camp a config.go\n2. Propagar-lo al client\n3. Escriure el test\nVERIFICACIÓ: go test ./..."
	mod, _ := m.answerPlan("e")
	mm := mod.(Model)
	items := tools.TodoList()
	if len(items) != 3 || items[0].Status != "working" || items[1].Status != "pending" {
		t.Fatalf("checklist: %+v", items)
	}
	if !mm.showTodos || !mm.agentActive {
		t.Fatalf("cockpit=%v agent=%v", mm.showTodos, mm.agentActive)
	}
	if mm.seedTodos != nil {
		t.Fatal("la llavor s'ha de consumir")
	}
	// La consigna diu al model que el checklist ja hi és.
	last := mm.histAgent()[len(mm.histAgent())-1].Content
	if !strings.Contains(last, "ja conté els passos numerats") || !strings.Contains(last, "Afegir el camp") {
		t.Fatalf("consigna: %q", last)
	}
	// Un torn nou sense pla neteja el checklist com sempre.
	mm.agentActive = false
	mm.busy = false
	mod2, _ := mm.startAgent("una altra cosa")
	if _, total := tools.TodoStats(); total != 0 || mod2.(Model).showTodos {
		t.Fatalf("el torn nou havia de començar net (total=%d)", total)
	}
}

// El checklist sempre té un pas en marxa mentre en quedin de pendents,
// encara que el model només marqui els fets.
func TestChecklistSempreTeUnPasEnMarxa(t *testing.T) {
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	tools.TodoSet([]tools.TodoItem{{Title: "u", Status: "done"}, {Title: "dos", Status: "pending"}, {Title: "tres", Status: "pending"}})
	items := tools.TodoList()
	if items[1].Status != "working" || items[2].Status != "pending" {
		t.Fatalf("estats: %+v", items)
	}
	tools.TodoSet([]tools.TodoItem{{Title: "u", Status: "done"}, {Title: "dos", Status: "done"}})
	if d, tot := tools.TodoStats(); d != 2 || tot != 2 {
		t.Fatalf("tot fet: %d/%d", d, tot)
	}
}
