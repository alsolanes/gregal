package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

// Patterns que el model escrivia de debò a les preguntes sobre aquest
// repositori i que, com a text literal, no trobaven res.
func TestGrepAcceptaRegexIEspais(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc (p *Policy) Decide(mode string) {}\n\nconst (\n\tModeCode    = \"code\"\n)\n")
	mustWrite(t, filepath.Join(dir, "b.go"), "package a\r\n\r\nfunc RunScouts() {}\r\n")
	cases := []struct {
		pattern, want string
	}{
		{"func (p *Policy) Decide", "a.go:3"},                // literal amb metacaràcters
		{`Decide\(`, "a.go:3"},                               // regex escapada
		{`func (p \*Policy) Decide|func.*Decide(`, "a.go:3"}, // alternança que no compila sencera
		{"ModeCode =", "a.go:6"},                             // alineat per gofmt
		{`func \(p \*Policy\) Decide`, "a.go:3"},             // escapat a mà
		{`Decide(mode|zzz`, "a.go:3"},                        // parèntesi sense tancar
		{`^func RunScouts\(\)`, "b.go:3"},                    // ancorat, fitxer CRLF
		{`ScoutWantedIn|RunScouts\(`, "b.go:3"},              // alternança vàlida
	}
	for _, c := range cases {
		out, err := Grep(c.pattern, dir, "", 50)
		if err != nil || !strings.Contains(out, c.want) {
			t.Errorf("Grep(%q) = %q, %v; volia %s", c.pattern, out, err, c.want)
		}
	}
	if out, _ := Grep("Decide(mode int", dir, "", 50); out != "sense coincidències" {
		t.Errorf("un literal que no hi és no ha de casar: %q", out)
	}
}

func TestCaminsRelatius(t *testing.T) {
	base := t.TempDir()
	abs := filepath.Join(base, "internal", "agent", "loop.go")
	out := CaminsRelatius(abs+":12:func x() {}\n"+abs+"\nsense prefix:1:y", base)
	want := "internal/agent/loop.go:12:func x() {}\ninternal/agent/loop.go\nsense prefix:1:y"
	if out != want {
		t.Fatalf("CaminsRelatius:\n%q\nvolia\n%q", out, want)
	}
	if got := CaminsRelatius("sense coincidències", base); got != "sense coincidències" {
		t.Fatalf("sense camins: %q", got)
	}
	if got := CaminsRelatius(abs, ""); got != abs {
		t.Fatalf("sense base no es toca: %q", got)
	}
}

// Una línia minificada no entra sencera: el tros del voltant de la
// coincidència i la mida.
func TestGrepTallaLiniesLlargues(t *testing.T) {
	dir := t.TempDir()
	llarga := strings.Repeat("a;", 20000) + "DoomGuide()" + strings.Repeat("b;", 20000)
	mustWrite(t, filepath.Join(dir, "x.min.js"), llarga+"\n")
	out, err := Grep("DoomGuide", dir, "", 50)
	if err != nil || !strings.Contains(out, "DoomGuide()") || !strings.Contains(out, "[line of 80011 characters]") {
		t.Fatalf("grep: %q %v", out, err)
	}
	if len(out) > maxLiniaGrep+200 {
		t.Fatalf("la línia no s'ha tallat: %d caràcters", len(out))
	}
}
