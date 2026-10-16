package llm

import (
	"encoding/json"
	"testing"
)

var tcSpecs = []ToolSpec{
	{Name: "read", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer"}}}},
	{Name: "bash", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"command": map[string]any{"type": "string"}}}},
	{Name: "office_edit", Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"track": map[string]any{"type": "boolean"}, "shapes": map[string]any{"type": "object"}}}},
}

func args(t *testing.T, tc ToolCall) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &m); err != nil {
		t.Fatalf("arguments invàlids %q: %v", tc.Function.Arguments, err)
	}
	return m
}

func TestRecoverHermesCall(t *testing.T) {
	text, calls := RecoverCalls("Ho llegeixo.\n<tool_call>\n{\"name\": \"read\", \"arguments\": {\"path\": \"a.go\", \"offset\": \"20\"}}\n</tool_call>", nil, tcSpecs)
	if len(calls) != 1 || calls[0].Function.Name != "read" {
		t.Fatalf("calls=%+v", calls)
	}
	a := args(t, calls[0])
	if a["path"] != "a.go" || a["offset"] != 20.0 {
		t.Fatalf("args=%v", a)
	}
	if text != "Ho llegeixo." {
		t.Fatalf("text=%q", text)
	}
}

func TestRecoverQwenXMLCall(t *testing.T) {
	in := "<tool_call>\n<function=bash>\n<parameter=command>\ngo test ./...\n</parameter>\n</function>\n</tool_call>"
	text, calls := RecoverCalls(in, nil, tcSpecs)
	if len(calls) != 1 || calls[0].Function.Name != "bash" || args(t, calls[0])["command"] != "go test ./..." || text != "" {
		t.Fatalf("text=%q calls=%+v", text, calls)
	}
}

func TestRecoverBareJSONOnlyWhenWholeMessage(t *testing.T) {
	_, calls := RecoverCalls(`{"name":"read","arguments":{"path":"x.md",}}`, nil, tcSpecs)
	if len(calls) != 1 || args(t, calls[0])["path"] != "x.md" {
		t.Fatalf("calls=%+v", calls)
	}
	// Dins d'una explicació no és una crida.
	text, calls := RecoverCalls("Per exemple: {\"name\":\"read\",\"arguments\":{}} fa una lectura.", nil, tcSpecs)
	if len(calls) != 0 || text == "" {
		t.Fatalf("no s'havia de convertir: %+v", calls)
	}
}

func TestUnknownToolIsNotExecuted(t *testing.T) {
	in := "<tool_call>{\"name\": \"rm_rf\", \"arguments\": {}}</tool_call>"
	text, calls := RecoverCalls(in, nil, tcSpecs)
	if len(calls) != 0 || text != in {
		t.Fatalf("una eina no oferta no es pot cridar: %+v", calls)
	}
	// Sense eines a la petició, mai.
	if _, calls := RecoverCalls("<tool_call>{\"name\": \"read\", \"arguments\": {}}</tool_call>", nil, nil); len(calls) != 0 {
		t.Fatal("sense specs no hi ha crides")
	}
}

func TestRepairNativeArguments(t *testing.T) {
	tc := ToolCall{Type: "function"}
	tc.Function.Name = "office_edit"
	tc.Function.Arguments = `{"track": "true", "shapes": "{\"title\": \"Hola\"}",}`
	out := RepairCalls([]ToolCall{tc}, tcSpecs)
	a := args(t, out[0])
	if a["track"] != true {
		t.Fatalf("track=%v", a["track"])
	}
	if sh, ok := a["shapes"].(map[string]any); !ok || sh["title"] != "Hola" {
		t.Fatalf("shapes=%v", a["shapes"])
	}
}
