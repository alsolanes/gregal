package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gregal/internal/backlog"
	"gregal/internal/llm"
)

// L'eina backlog: el model llegeix i mou les tasques del projecte amb les
// mateixes regles que la UI (internal/backlog). Crear i apuntar al log es
// pot fer des de qualsevol mode (és com prendre nota); la resta escriu i
// només va en els modes que actuen.

func backlogToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	num := map[string]any{"type": "integer"}
	return llm.ToolSpec{Name: "backlog", Description: "The project's backlog of work items (feature, bug, task, spike, epic), stored by Gregal, with no forge needed. " +
		"Open the work-items skill before creating or closing items. Actions: " +
		"list (optional status, type); get (id); " +
		"create (type, title, why, criteria as a list of checkable statements, out_of_scope, priority now|next|later, size S|M|L, parent epic id, depends_on ids; always starts as proposed); " +
		"edit (id and the fields to change); " +
		"move (id, status proposed|ready|in_progress|in_review|done|blocked|dropped, reason required for blocked and dropped); " +
		"check (id, n = criterion number from 1, evidence required; done=false unticks); " +
		"log (id, text). " +
		"Rules: start only ready items, work only on their criteria, tick each criterion with evidence, end in in_review. Never move to done: the person or the reviewer does.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"list", "get", "create", "edit", "move", "check", "log"}},
			"id":     num, "n": num, "parent": num,
			"type": str, "title": str, "why": str, "out_of_scope": str, "priority": str, "size": str,
			"status": str, "reason": str, "evidence": str, "text": str,
			"done":       map[string]any{"type": "boolean"},
			"criteria":   map[string]any{"type": "array", "items": str},
			"depends_on": map[string]any{"type": "array", "items": num},
		}, "required": []string{"action"}}}
}

type backlogArgs struct {
	Action     string   `json:"action"`
	ID         int      `json:"id"`
	N          int      `json:"n"`
	Parent     *int     `json:"parent"`
	Type       *string  `json:"type"`
	Title      *string  `json:"title"`
	Why        *string  `json:"why"`
	OutOfScope *string  `json:"out_of_scope"`
	Priority   *string  `json:"priority"`
	Size       *string  `json:"size"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason"`
	Evidence   string   `json:"evidence"`
	Text       string   `json:"text"`
	Done       *bool    `json:"done"`
	Criteria   []string `json:"criteria"`
	DependsOn  []int    `json:"depends_on"`
}

// BacklogWrites diu si una crida a l'eina canvia més que una nota: crear i
// apuntar al log es permeten també als modes de només lectura.
func BacklogWrites(argsJSON string) bool {
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "list", "get", "create", "log", "":
		return false
	}
	return true
}

// einaBacklog és una crida a l'eina: el backlog del projecte, qui consta
// al log i on treballa l'agent.
type einaBacklog struct {
	s   *backlog.Store
	a   backlogArgs
	who string
	dir string
}

func backlogExec(dir, session, argsJSON string) (string, error) {
	var a backlogArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	s, err := backlog.Open(procDirOr(dir))
	if err != nil {
		return "", err
	}
	e := einaBacklog{s: s, a: a, who: "agent", dir: procDirOr(dir)}
	if session != "" {
		e.who += " · " + session
	}
	e.a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	switch e.a.Action {
	case "list":
		return e.list()
	case "get":
		return e.get()
	}
	accions := map[string]func() (backlog.Item, error){
		"create": e.create, "edit": e.edit, "move": e.move, "check": e.check, "log": e.log,
	}
	fer, ok := accions[e.a.Action]
	if !ok {
		return "", fmt.Errorf("unknown action %q: list, get, create, edit, move, check or log", e.a.Action)
	}
	if e.a.Action != "create" && e.a.ID <= 0 {
		return "", fmt.Errorf("%s needs the item id", e.a.Action)
	}
	it, err := fer()
	if err != nil {
		return "", err
	}
	out := "OK " + backlog.Resum(it)
	if it.Status == backlog.StatusInReview && backlog.OnInReview != nil {
		out += "\nAn independent reviewer is now checking it against the criteria; its verdict goes to the item's log. If it is NOT MET the item comes back to in_progress with what is missing."
	}
	return out, nil
}

func (e einaBacklog) list() (string, error) {
	all, err := e.s.List()
	if err != nil {
		return "", err
	}
	return backlogList(e.s, all, e.a.Status, deref(e.a.Type)), nil
}

func (e einaBacklog) get() (string, error) {
	if e.a.ID <= 0 {
		return "", errors.New("get needs the item id")
	}
	it, err := e.s.Get(e.a.ID)
	if err != nil {
		return "", err
	}
	return backlog.Markdown(it), nil
}

func (e einaBacklog) create() (backlog.Item, error) {
	nou := backlog.Item{DependsOn: e.a.DependsOn}
	aplicaCamps(&nou, e.a)
	return e.s.Create(nou, e.who)
}

func (e einaBacklog) edit() (backlog.Item, error) {
	return e.s.Edit(e.a.ID, e.who, func(i *backlog.Item) {
		aplicaCamps(i, e.a)
		if e.a.DependsOn != nil {
			i.DependsOn = e.a.DependsOn
		}
	})
}

func (e einaBacklog) move() (backlog.Item, error) {
	if strings.EqualFold(strings.TrimSpace(e.a.Status), backlog.StatusDone) {
		return backlog.Item{}, fmt.Errorf("an agent does not close items: move #%d to in_review and the person or the reviewer marks it done", e.a.ID)
	}
	it, err := e.s.Move(e.a.ID, e.a.Status, e.a.Reason, e.who)
	if err == nil && it.Status == backlog.StatusInProgress && it.Workdir == "" {
		// Començada des d'aquí: el revisor mirarà els canvis d'aquesta
		// carpeta des d'aquest commit.
		it, err = e.s.SetWork(e.a.ID, e.dir, GitHead(e.dir))
	}
	return it, err
}

func (e einaBacklog) check() (backlog.Item, error) {
	done := e.a.Done == nil || *e.a.Done
	return e.s.Check(e.a.ID, e.a.N, done, e.a.Evidence, e.who)
}

func (e einaBacklog) log() (backlog.Item, error) {
	return e.s.AddLog(e.a.ID, e.a.Text, e.who)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func aplicaCamps(it *backlog.Item, a backlogArgs) {
	if a.Type != nil {
		it.Type = *a.Type
	}
	if a.Title != nil {
		it.Title = *a.Title
	}
	if a.Why != nil {
		it.Why = *a.Why
	}
	if a.OutOfScope != nil {
		it.OutOfScope = *a.OutOfScope
	}
	if a.Priority != nil {
		it.Priority = *a.Priority
	}
	if a.Size != nil {
		it.Size = *a.Size
	}
	if a.Parent != nil {
		it.Parent = *a.Parent
	}
	if a.Criteria != nil {
		// Els criteris que ja hi eren conserven la marca i la prova si el
		// text no canvia.
		prev := map[string]backlog.Criterion{}
		for _, c := range it.Criteria {
			prev[c.Text] = c
		}
		it.Criteria = it.Criteria[:0:0]
		for _, t := range a.Criteria {
			c, ok := prev[strings.TrimSpace(t)]
			if !ok {
				c = backlog.Criterion{Text: t}
			}
			it.Criteria = append(it.Criteria, c)
		}
	}
}

func backlogList(s *backlog.Store, all []backlog.Item, status, tipus string) string {
	status = strings.NewReplacer(" ", "_", "-", "_").Replace(strings.ToLower(strings.TrimSpace(status)))
	tipus = strings.ToLower(strings.TrimSpace(tipus))
	var b strings.Builder
	fmt.Fprintf(&b, "Backlog of %s (%s)\n", s.Root, s.Storage)
	n := 0
	for _, it := range all {
		if status != "" && it.Status != status {
			continue
		}
		if status == "" && (it.Status == backlog.StatusDone || it.Status == backlog.StatusDropped) {
			continue
		}
		if tipus != "" && it.Type != tipus {
			continue
		}
		b.WriteString(backlog.Resum(it) + "\n")
		n++
	}
	if n == 0 {
		b.WriteString("No items")
		if status == "" {
			b.WriteString(" open (done and dropped are hidden; pass status to see them)")
		}
		b.WriteString(".\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
