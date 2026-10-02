package tools

// ParseQuestion valida els args de l'eina question: query + 1-4 opcions
// amb label (descripció opcional). La UI (TUI/web) ho pinta seleccionable;
// sense UI, l'agent rep guia per demanar per text.

import (
	"encoding/json"
	"fmt"
	"strings"
)

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ParseQuestion retorna query neta + opcions validades.
func ParseQuestion(argsJSON string) (string, []QuestionOption, error) {
	var a struct {
		Query    string          `json:"query"`
		Question string          `json:"question"`
		Options  json.RawMessage `json:"options"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", nil, err
	}
	q := strings.TrimSpace(a.Query)
	if q == "" {
		q = strings.TrimSpace(a.Question)
	}
	if q == "" {
		return "", nil, fmt.Errorf("query buida")
	}

	// 1. Intentem com a []QuestionOption (comprovem que tinguin label)
	var objOpts []QuestionOption
	if err := json.Unmarshal(a.Options, &objOpts); err == nil && len(objOpts) > 0 {
		hasLabel := true
		for _, o := range objOpts {
			if strings.TrimSpace(o.Label) == "" {
				hasLabel = false
				break
			}
		}
		if hasLabel {
			return validateOptions(q, objOpts)
		}
	}

	// 2. Intentem com a []string (comú en models locals petits)
	var strOpts []string
	if err := json.Unmarshal(a.Options, &strOpts); err == nil && len(strOpts) > 0 {
		opts := make([]QuestionOption, 0, len(strOpts))
		for _, s := range strOpts {
			opts = append(opts, QuestionOption{Label: s})
		}
		return validateOptions(q, opts)
	}

	// 3. Intentem com a []map[string]any
	var mapOpts []map[string]any
	if err := json.Unmarshal(a.Options, &mapOpts); err == nil && len(mapOpts) > 0 {
		opts := make([]QuestionOption, 0, len(mapOpts))
		for _, m := range mapOpts {
			lbl := ""
			for _, k := range []string{"label", "text", "value", "name", "option"} {
				if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
					lbl = v
					break
				}
			}
			desc, _ := m["description"].(string)
			opts = append(opts, QuestionOption{Label: lbl, Description: desc})
		}
		return validateOptions(q, opts)
	}

	return "", nil, fmt.Errorf("calen 1-4 opcions")
}

func validateOptions(q string, raw []QuestionOption) (string, []QuestionOption, error) {
	if len(raw) == 0 || len(raw) > 4 {
		return "", nil, fmt.Errorf("calen 1-4 opcions (tinc %d)", len(raw))
	}
	out := make([]QuestionOption, 0, len(raw))
	for _, o := range raw {
		l := strings.TrimSpace(o.Label)
		if l == "" {
			return "", nil, fmt.Errorf("opció sense label")
		}
		if r := []rune(l); len(r) > 60 {
			l = string(r[:60])
		}
		d := strings.TrimSpace(o.Description)
		if r := []rune(d); len(r) > 120 {
			d = string(r[:120])
		}
		out = append(out, QuestionOption{Label: l, Description: d})
	}
	if r := []rune(q); len(r) > 500 {
		q = string(r[:500])
	}
	return q, out, nil
}
