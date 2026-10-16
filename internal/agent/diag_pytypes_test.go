package agent

import (
	"strings"
	"testing"
)

func TestParsePyright(t *testing.T) {
	raw := `{"generalDiagnostics":[
	 {"file":"C:\\p\\app.py","severity":"error","message":"Argument of type \"str\" cannot be assigned to parameter \"n\" of type \"int\"\n  \"str\" is not assignable to \"int\"","range":{"start":{"line":4}}},
	 {"file":"C:\\p\\app.py","severity":"warning","message":"unused","range":{"start":{"line":1}}},
	 {"file":"C:\\p\\other.py","severity":"error","message":"elsewhere","range":{"start":{"line":0}}}]}`
	got := parsePyright(raw, "app.py")
	if !strings.Contains(got, "app.py:5: Argument of type \"str\"") || strings.Contains(got, "unused") || strings.Contains(got, "elsewhere") || strings.Contains(got, "assignable") {
		t.Fatalf("parse: %q", got)
	}
	if parsePyright(`{"generalDiagnostics":[]}`, "app.py") != "" {
		t.Fatal("sense errors ha de callar")
	}
}
