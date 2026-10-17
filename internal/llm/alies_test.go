package llm

import "testing"

var aliesSpecs = []ToolSpec{
	{Name: "edit", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"path": map[string]any{"type": "string"}, "old_string": map[string]any{"type": "string"},
		"new_string": map[string]any{"type": "string"}, "replace_all": map[string]any{"type": "boolean"}}}},
	{Name: "patch", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"path": map[string]any{"type": "string"},
		"edits": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
			"old": map[string]any{"type": "string"}, "new": map[string]any{"type": "string"},
			"after_line": map[string]any{"type": "integer"}}}}}}},
	{Name: "bash", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"command": map[string]any{"type": "string"}}}},
}

func crida(name, a string) []ToolCall {
	var c ToolCall
	c.Type, c.Function.Name, c.Function.Arguments = "function", name, a
	return []ToolCall{c}
}

// El cas del banc: edit amb les claus del patch.
func TestAliesEditAmbClausDelPatch(t *testing.T) {
	c := RepairCalls(crida("edit", `{"path":"s.go","old":"a, _ := f()","new":"a, i := f()"}`), aliesSpecs)
	m := args(t, c[0])
	if m["old_string"] != "a, _ := f()" || m["new_string"] != "a, i := f()" || m["old"] != nil {
		t.Fatalf("args=%v", m)
	}
}

func TestAliesDAltresArnesos(t *testing.T) {
	for _, a := range []string{
		`{"file_path":"s.go","old_string":"x","new_string":"y","replaceAll":"true"}`, // Claude Code
		`{"filePath":"s.go","oldString":"x","newString":"y","replaceAll":true}`,      // opencode
		`{"path":"s.go","old_str":"x","new_str":"y","replace_all":true}`,             // str_replace
	} {
		m := args(t, RepairCalls(crida("edit", a), aliesSpecs)[0])
		if m["path"] != "s.go" || m["old_string"] != "x" || m["new_string"] != "y" || m["replace_all"] != true {
			t.Errorf("%s → %v", a, m)
		}
	}
	if m := args(t, RepairCalls(crida("bash", `{"cmd":"ls"}`), aliesSpecs)[0]); m["command"] != "ls" {
		t.Errorf("bash cmd → %v", m)
	}
}

// Dins de les llistes d'objectes (edits del patch) també.
func TestAliesDinsDeLlistes(t *testing.T) {
	c := RepairCalls(crida("patch", `{"path":"s.go","edits":[{"old_string":"a","new_string":"b"},{"old":"c","new":"d"}]}`), aliesSpecs)
	m := args(t, c[0])
	eds := m["edits"].([]any)
	e0 := eds[0].(map[string]any)
	if e0["old"] != "a" || e0["new"] != "b" || e0["old_string"] != nil {
		t.Fatalf("edits=%v", eds)
	}
}

// Un àlies no trepitja mai un paràmetre que ja hi és, i una clau
// desconeguda que no és cap àlies es deixa tal qual.
func TestAliesNoTrepitja(t *testing.T) {
	m := args(t, RepairCalls(crida("edit", `{"path":"s.go","old_string":"x","old":"z","new_string":"y","colour":"red"}`), aliesSpecs)[0])
	if m["old_string"] != "x" || m["old"] != "z" || m["colour"] != "red" {
		t.Fatalf("args=%v", m)
	}
}
