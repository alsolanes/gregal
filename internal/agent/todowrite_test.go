package agent

import (
	"testing"

	"gregal/internal/tools"
)

// Payload literal emès pel Qwen en viu (via strix): "completed" i
// "in_progress" han de comptar com a fet i en marxa, o el checklist del
// TUI es queda clavat a 0/N per sempre.
func TestTodowriteQwenCompta(t *testing.T) {
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	args := `{"items": [{"title": "Llegir fitxer", "status": "completed"}, {"title": "Escriure codi", "status": "in_progress"}]}`
	out, _, err := Exec("todowrite", args)
	if err != nil {
		t.Fatal(err)
	}
	done, total := tools.TodoStats()
	if done != 1 || total != 2 {
		t.Fatalf("done=%d total=%d (sortida: %q)", done, total, out)
	}
}
