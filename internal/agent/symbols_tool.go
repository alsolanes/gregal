package agent

import (
	"encoding/json"
	"strings"

	"gregal/internal/llm"
	"gregal/internal/repomap"
)

// L'eina `symbols`: cerca de definicions a l'índex de repomap. És de
// lectura pura (com grep i glob) i es pot fer servir als modes de només
// lectura i en paral·lel amb altres lectures.

func symbolsToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{Name: "symbols", Description: "Find where things are defined: searches the project's symbol index (top-level functions, methods, types, classes, interfaces, enums, traits, consts, vars in Go, JS/TS, Python, Java, C#, Rust) and returns path:line kind name — signature. query is a case-insensitive substring of the name (Type.method also works) or a regular expression with regex=true; filter with kind and file (glob such as internal/**/*.go or *_test.go). Use it before grep when you look for a definition.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"query":    str,
			"regex":    map[string]any{"type": "boolean"},
			"kind":     map[string]any{"type": "string", "enum": []string{"func", "method", "type", "struct", "interface", "class", "enum", "trait", "impl", "const", "var"}},
			"file":     str,
			"exported": map[string]any{"type": "boolean"},
			"dir":      str,
			"limit":    map[string]any{"type": "integer"},
		}}}
}

// symbolsTool executa la cerca. dir és el workspace de la sessió (o "" pel
// del procés); l'argument "dir" ja arriba resolt per resolArgs.
func symbolsTool(dir, argsJSON string) (string, error) {
	var a struct {
		Query    string `json:"query"`
		Name     string `json:"name"`
		Regex    bool   `json:"regex"`
		Kind     string `json:"kind"`
		File     string `json:"file"`
		Exported bool   `json:"exported"`
		Dir      string `json:"dir"`
		Limit    int    `json:"limit"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
	}
	arrel := a.Dir
	if strings.TrimSpace(arrel) == "" {
		arrel = procDirOr(dir)
	}
	hits, snap, err := repomap.Search(arrel, repomap.Query{
		Name: firstOf(a.Query, a.Name), Regex: a.Regex, Kind: a.Kind, File: a.File, Exported: a.Exported,
	})
	if err != nil {
		return "", err
	}
	return repomap.FormatHits(hits, snap, a.Limit), nil
}
