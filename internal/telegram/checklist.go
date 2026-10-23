package telegram

import (
	"encoding/json"
	"sync"

	"gregal/internal/tools"
)

// A checklist belongs to the current request, not the Telegram process.
// Explicit continuations retain conversation history and may create a new
// checklist; no old or concurrent chat may impose completion on this turn.
type turnChecklist struct {
	mu    sync.Mutex
	items []tools.TodoItem
}

func (c *turnChecklist) list() []tools.TodoItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]tools.TodoItem(nil), c.items...)
}

// exec intercepts bookkeeping before ExecCtx can touch the global TUI store.
func (c *turnChecklist) exec(name, args string) (string, bool, error) {
	switch name {
	case "todowrite":
		var input struct {
			Items []tools.TodoItem `json:"items"`
		}
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			return "", true, err
		}
		items := tools.NetejaTodos(input.Items)
		c.mu.Lock()
		c.items = items
		c.mu.Unlock()
		return tools.RenderTodos(items), true, nil
	case "todoread":
		return tools.RenderTodos(c.list()), true, nil
	default:
		return "", false, nil
	}
}
