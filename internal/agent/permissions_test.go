package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSigEstable(t *testing.T) {
	w1 := Sig("write", `{"path":"pkg/main.go","content":"a"}`)
	w2 := Sig("write", `{"path":"pkg/main.go","content":"b totalment diferent"}`)
	if w1 != w2 {
		t.Fatalf("mateix fitxer hauria de compartir sig: %q vs %q", w1, w2)
	}
	wSameDir := Sig("write", `{"path":"pkg/altre.go","content":"a"}`)
	if w1 != wSameDir {
		t.Fatalf("mateix directori hauria de compartir sig: %q vs %q", w1, wSameDir)
	}
	w3 := Sig("write", `{"path":"altrepaquet/f.go","content":"a"}`)
	if w1 == w3 {
		t.Fatalf("directori diferent no pot compartir sig: %q", w1)
	}
	b1 := Sig("bash", `{"command":"git status"}`)
	b2 := Sig("bash", `{"command":"git status"}`)
	if b1 != b2 {
		t.Fatalf("bash idèntic hauria de compartir sig")
	}
	m1 := Sig("mcp_recordatoris_afegeix", `{"a":1}`)
	m2 := Sig("mcp_recordatoris_afegeix", `{"a":2,"b":3}`)
	if m1 != m2 {
		t.Fatalf("MCP normalitza per eina: %q vs %q", m1, m2)
	}
}

func TestReadOnlyDoesNotInheritMutationGrants(t *testing.T) {
	p := &Policy{Tools: map[string]string{"bash": "allow", "browser": "allow"}, BashAllow: []string{"python"}}
	for _, mode := range []string{ModeInspect, ModeChat, ModeGoal} {
		for _, call := range []struct{ name, args string }{
			{"office_edit", `{}`},
			{"office_create", `{}`},
			{"bash_kill", `{}`},
			{"bash", `{"command":"python -c 'print(1)'"}`},
			{"browser", `{"action":"click"}`},
			{"browser", `{"action":"eval"}`},
		} {
			if d, _ := p.Decide(mode, call.name, call.args); d != "deny" {
				t.Fatalf("%s %s: got %s", mode, call.name, d)
			}
		}
		if d, _ := p.Decide(mode, "bash", `{"command":"git status"}`); d != "allow" {
			t.Fatalf("%s: safe read blocked", mode)
		}
	}
}

func TestBashAliasesShareStrictestExplicitPermission(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]string
		args     string
		want     string
	}{
		{"bash ask reaches background", map[string]string{"bash": "ask"}, `{"command":"go test ./..."}`, "ask"},
		{"bash deny reaches background", map[string]string{"bash": "deny"}, `{"command":"go test ./..."}`, "deny"},
		{"background ask reaches bash", map[string]string{"bash_background": "ask"}, `{"command":"go test ./..."}`, "ask"},
		{"background deny reaches bash", map[string]string{"bash_background": "deny"}, `{"command":"go test ./..."}`, "deny"},
		{"ask wins over allow", map[string]string{"bash": "allow", "bash_background": "ask"}, `{"command":"go test ./..."}`, "ask"},
		{"deny wins over allow", map[string]string{"bash": "allow", "bash_background": "deny"}, `{"command":"go test ./..."}`, "deny"},
		{"allow cannot relax hard deny", map[string]string{"bash": "allow", "bash_background": "allow"}, `{"command":"sudo rm -rf /tmp/x"}`, "deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Policy{Tools: tc.settings}
			for _, mode := range []string{ModeCode, ModeAutonomous} {
				for _, tool := range []string{"bash", "bash_background"} {
					if got, _ := p.Decide(mode, tool, tc.args); got != tc.want {
						t.Errorf("%s %s = %s; want %s", mode, tool, got, tc.want)
					}
				}
			}
		})
	}
}

func TestTodoBookkeepingAllowedInAllModes(t *testing.T) {
	p := DefaultPolicy()
	for _, mode := range []string{ModeCode, ModeAutonomous, ModeChat, ModeInspect, ModeGoal} {
		for _, tool := range []string{"todoread", "todowrite"} {
			if got, _ := p.Decide(mode, tool, `{}`); got != "allow" {
				t.Errorf("%s %s = %s; want allow", mode, tool, got)
			}
		}
	}
}

func TestRememberAmbits(t *testing.T) {
	r := NewRemember()
	if r.Allowed("web", "write\x00main.go") {
		t.Fatalf("res recordat encara")
	}
	r.Allow("web", "write\x00main.go")
	if !r.Allowed("web", "write\x00main.go") {
		t.Fatalf("hauria d'estar permès")
	}
	if r.Allowed("tui", "write\x00main.go") {
		t.Fatalf("els àmbits són independents")
	}
	r.Clear("web")
	if r.Allowed("web", "write\x00main.go") {
		t.Fatalf("Clear ha de netejar l'àmbit")
	}
	// Nil-safe: els frontends poden tenir remembers a nil en tests.
	var nilR *Remember
	nilR.Allow("web", "x")
	if nilR.Allowed("web", "x") {
		t.Fatalf("nil mai permet")
	}
	nilR.Clear("web")
}

func TestInsideProject(t *testing.T) {
	dir := "/tmp/gregal-proj-test"
	casos := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"sub/dir/f.go", true},
		{dir + "/a.go", true},
		{dir, true},
		{"../fora.go", false},
		{dir + "/../fora.go", false},
		{"/etc/passwd", false},
		{`\Windows\System32\drivers\etc\hosts`, false}, // arrel sense unitat
		{"C:fitxer", false},                            // relatiu a una altra unitat
		{dir + "2/germana.go", false},                  // germà amb prefix comú, no fill
		{"~/secret.go", false},
		{"", false},
		{".", true},
	}
	for _, c := range casos {
		if got := InsideProject(dir, c.path); got != c.want {
			t.Errorf("InsideProject(%q, %q) = %v, volia %v", dir, c.path, got, c.want)
		}
	}
	if InsideProject("", "main.go") {
		t.Errorf("sense projecte no hi ha pas automàtic")
	}
}

// Un directori amb enllaç o nom curt 8.3 (RUNNER~1 al CI de Windows) es
// resol diferent de com s'escriu: la relativa s'ha d'enganxar al directori
// JA resolt, o el prefix no coincideix i tot demana permís (torn penjat
// 30 min esperant una aprovació que no vindrà).
func TestInsideProjectEnllacOSymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("sense privilegis per crear enllaços: %v", err)
	}
	if !InsideProject(link, "a.txt") {
		t.Errorf("relativa dins un dir amb enllaç ha de passar")
	}
	if !InsideProject(link, filepath.Join(link, "sub", "b.txt")) {
		t.Errorf("absoluta escrita via l'enllaç ha de passar")
	}
	if InsideProject(link, filepath.Join(base, "fora.txt")) {
		t.Errorf("el germà de l'enllaç no hi és dins")
	}
}

func TestDecideProjecte(t *testing.T) {
	p := &Policy{ProjectDir: "/tmp/gregal-proj-test"}
	w := `{"path":"main.go","content":"x"}`
	if d, _ := p.Decide(ModeCode, "write", w); d != "allow" {
		t.Errorf("code+write dins projecte = %q, volia allow", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "write", w); d != "allow" {
		t.Errorf("autonomous+write dins projecte = %q, volia allow", d)
	}
	if d, _ := p.Decide(ModeCode, "edit", `{"path":"sub/f.go","old_string":"a","new_string":"b"}`); d != "allow" {
		t.Errorf("code+edit dins projecte = %q, volia allow", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "patch", `{"path":"sub/f.go","edits":[]}`); d != "allow" {
		t.Errorf("autonomous+patch dins projecte = %q, volia allow", d)
	}
	if d, _ := p.Decide(ModeCode, "write", `{"path":"/etc/hosts","content":"x"}`); d != "ask" {
		t.Errorf("fora del projecte continua demanant: %q", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "write", `{"path":"/etc/hosts","content":"x"}`); d != "ask" {
		t.Errorf("autonomous fora del projecte continua demanant: %q", d)
	}
	if d, _ := p.Decide(ModeChat, "write", w); d != "deny" {
		t.Errorf("xat continua sent només lectura: %q", d)
	}
	if d, _ := p.Decide(ModeInspect, "write", w); d != "deny" {
		t.Errorf("consulta no modifica mai: %q", d)
	}
	if d, _ := p.Decide(ModeGoal, "write", w); d != "deny" {
		t.Errorf("objectiu no modifica fins executar: %q", d)
	}
	if d, _ := p.Decide(ModeChat, "patch", `{"path":"main.go","edits":[]}`); d != "deny" {
		t.Errorf("xat continua sent només lectura per patch: %q", d)
	}
	if d, _ := p.Decide(ModeInspect, "patch", `{"path":"main.go","edits":[]}`); d != "deny" {
		t.Errorf("consulta no modifica per patch: %q", d)
	}
	sense := &Policy{}
	if d, _ := sense.Decide(ModeCode, "write", w); d != "ask" {
		t.Errorf("sense ProjectDir es conserva l'ask: %q", d)
	}
	override := &Policy{ProjectDir: "/tmp/gregal-proj-test", Tools: map[string]string{"write": "ask"}}
	if d, _ := override.Decide(ModeCode, "write", w); d != "ask" {
		t.Errorf("l'override explícit del config guanya a l'automatisme: %q", d)
	}
	if d, _ := override.Decide(ModeAutonomous, "write", w); d != "ask" {
		t.Errorf("l'override explícit del config guanya també en autònom: %q", d)
	}
}
