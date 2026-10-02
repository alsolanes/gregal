package tools

import "testing"

// Vist en viu amb un Qwen: actualitza amb "completed"/"in_progress" i
// el checklist es quedava a 0/N perquè tot queia a pending.
func TestEstatsAliasArriben(t *testing.T) {
	TodoSet([]TodoItem{
		{Title: "llegit", Status: "completed"},
		{Title: "fent", Status: "in_progress"},
		{Title: "pendent", Status: "pending"},
		{Title: "estrany", Status: "gairebé"},
	})
	t.Cleanup(TodoClear)
	done, total := TodoStats()
	if done != 1 || total != 4 {
		t.Fatalf("done=%d total=%d, volia 1/4", done, total)
	}
	got := TodoList()
	if got[0].Status != "done" || got[1].Status != "working" || got[2].Status != "pending" || got[3].Status != "pending" {
		t.Fatalf("estats mal normalitzats: %+v", got)
	}
	// NetejaTodos (via web, per sessió) fa el mateix sense global.
	netejen := NetejaTodos([]TodoItem{{Title: "x", Status: "COMPLETED"}, {Title: "y", Status: "Doing"}})
	if netejen[0].Status != "done" || netejen[1].Status != "working" {
		t.Fatalf("NetejaTodos no normalitza: %+v", netejen)
	}
}
