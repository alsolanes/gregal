package tools

// Todo amb checks (estil opencode todowrite/todoread): l'agent publica per
// on va la tasca i la UI ho pinta amb ✓/◌/☐. Magatzem en memòria amb mutex:
// un sol procés serveix TUI i web; en canviar de sessió web es neteja.

import (
	"fmt"
	"strings"
	"sync"
)

type TodoItem struct {
	Title  string `json:"title"`
	Status string `json:"status"` // "pending" | "working" | "done"
}

var todoMu sync.Mutex
var todoItems []TodoItem

// normalitzaEstat unifica el vocabulari d'estats: els models petits no
// sempre diuen pending/working/done (vist en viu: "completed" i
// "in_progress" d'un Qwen, que abans queien a pending i el checklist no
// avançava mai). S'aplica a TodoSet i NetejaTodos.
func normalitzaEstat(st string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(st, "-", "_"))) {
	case "done", "completed", "complete", "finished", "resolved", "success":
		return "done"
	case "working", "in_progress", "doing", "started", "active":
		return "working"
	default:
		return "pending"
	}
}

// TodoSet substitueix la llista (máx 20, títols retallats a 120 runes).
func TodoSet(items []TodoItem) {
	todoMu.Lock()
	defer todoMu.Unlock()
	if len(items) > 20 {
		items = items[:20]
	}
	out := make([]TodoItem, 0, len(items))
	for _, it := range items {
		t := strings.TrimSpace(it.Title)
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > 120 {
			t = string(r[:120])
		}
		out = append(out, TodoItem{Title: t, Status: normalitzaEstat(it.Status)})
	}
	todoItems = marcaEnMarxa(out)
}

// marcaEnMarxa garanteix que, si queden passos pendents i cap no és en
// marxa, el primer pendent passi a working: el cockpit sempre diu quin és
// el pas actual, encara que el model només hagi marcat el fet.
func marcaEnMarxa(items []TodoItem) []TodoItem {
	for _, it := range items {
		if it.Status == "working" {
			return items
		}
	}
	for i := range items {
		if items[i].Status == "pending" {
			items[i].Status = "working"
			break
		}
	}
	return items
}

// TodoList retorna una còpia.
func TodoList() []TodoItem {
	todoMu.Lock()
	defer todoMu.Unlock()
	return append([]TodoItem(nil), todoItems...)
}

// TodoClear buida (canvi de sessió).
func TodoClear() {
	todoMu.Lock()
	defer todoMu.Unlock()
	todoItems = nil
}

// TodoStats retorna fets i totals per la barra d'estat.
func TodoStats() (done, total int) {
	todoMu.Lock()
	defer todoMu.Unlock()
	total = len(todoItems)
	for _, it := range todoItems {
		if it.Status == "done" {
			done++
		}
	}
	return done, total
}

// TodoRender pinta la checklist per al xat/activitat.
func TodoRender() string {
	done, total := TodoStats()
	if total == 0 {
		return "cap todo (todowrite buit)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "todos %d/%d:\n", done, total)
	for _, it := range TodoList() {
		mark := "☐"
		switch it.Status {
		case "done":
			mark = "✓"
		case "working":
			mark = "◌"
		}
		b.WriteString(mark + " " + it.Title + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- Versions sense estat global ---
//
// El magatzem de dalt és un de sol per a tot el procés, i això va bé amb un
// TUI (una sessió) però no a l'escriptori amb dues pestanyes: la sessió que
// comença un torn esborrava la llista de la que estava treballant, i un
// todoread podia tornar la checklist de l'altra. Aquestes funcions treballen
// sobre una llista que et guardes tu, i cada sessió web es queda la seva.

// NetejaTodos aplica els mateixos límits que TodoSet (20 ítems, 120 runes,
// estat vàlid) sense tocar res de global.
func NetejaTodos(items []TodoItem) []TodoItem {
	if len(items) > 20 {
		items = items[:20]
	}
	out := make([]TodoItem, 0, len(items))
	for _, it := range items {
		t := strings.TrimSpace(it.Title)
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > 120 {
			t = string(r[:120])
		}
		out = append(out, TodoItem{Title: t, Status: normalitzaEstat(it.Status)})
	}
	return marcaEnMarxa(out)
}

// StatsTodos compta fets i totals d'una llista donada.
func StatsTodos(items []TodoItem) (done, total int) {
	total = len(items)
	for _, it := range items {
		if it.Status == "done" {
			done++
		}
	}
	return done, total
}

// RenderTodos pinta una llista donada.
func RenderTodos(items []TodoItem) string {
	done, total := StatsTodos(items)
	if total == 0 {
		return "cap todo (todowrite buit)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "todos %d/%d:\n", done, total)
	for _, it := range items {
		mark := "☐"
		switch it.Status {
		case "done":
			mark = "✓"
		case "working":
			mark = "◌"
		}
		b.WriteString(mark + " " + it.Title + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
