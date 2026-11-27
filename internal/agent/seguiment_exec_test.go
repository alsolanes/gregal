package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El camí de debò (ExecIn): read recorda què ha llegit la sessió, edit hi
// aplica la guarda i torna el diff, i una relectura sense canvis no repeteix
// el fitxer.
func TestExecReadEditAmbSeguiment(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("package a\n\nfunc F() {\n\tg()\n}\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "// comentari de farciment %d\n", i)
	}
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	sess := "seguiment-" + t.Name()

	// Sense llegir, l'edició aproximada no passa.
	if _, _, err := ExecIn(sess, dir, "edit", `{"path":"a.go","old_string":"func F() {\n    g()\n}","new_string":"func F() {\n    h()\n}"}`); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("havia de demanar read: %v", err)
	}
	if out, _, err := ExecIn(sess, dir, "read", `{"path":"a.go"}`); err != nil || !strings.Contains(out, "4|\tg()") {
		t.Fatalf("read: %v %q", err, out)
	}
	if out, _, _ := ExecIn(sess, dir, "read", `{"path":"a.go"}`); !strings.Contains(out, "unchanged:") {
		t.Fatalf("la relectura havia de dir sense canvis: %q", out)
	}
	out, _, err := ExecIn(sess, dir, "edit", `{"path":"a.go","old_string":"func F() {\n    g()\n}","new_string":"func F() {\n    h()\n}"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "approximate") || !strings.Contains(out, "+4|\th()") {
		t.Errorf("edit ha de dir el nivell i donar el diff: %q", out)
	}
	if raw, _ := os.ReadFile(p); !strings.Contains(string(raw), "\th()") {
		t.Errorf("%q", raw)
	}
}

// Un write que arriba sense content (la resposta del model s'ha tallat pel
// límit de sortida i només en queda el path) no pot buidar el fitxer: torna
// un error que diu què ha passat i com sortir-se'n.
func TestExecWriteSenseContentNoBuidaElFitxer(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "index.html")
	if err := os.WriteFile(p, []byte("<p>hola</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sess := "write-sense-content-" + t.Name()
	if _, _, err := ExecIn(sess, dir, "read", `{"path":"index.html"}`); err != nil {
		t.Fatal(err)
	}
	_, _, err := ExecIn(sess, dir, "write", `{"path":"index.html"}`)
	if err == nil {
		t.Fatal("un write sense content havia de fallar")
	}
	for _, w := range []string{"content", "output token limit", "edit"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("l'error no diu %q: %v", w, err)
		}
	}
	if raw, _ := os.ReadFile(p); string(raw) != "<p>hola</p>\n" {
		t.Fatalf("el fitxer ha canviat: %q", raw)
	}
}

// content "" explícit és una ordre de debò: crea un fitxer buit.
func TestExecWriteContentBuitCreaFitxerBuit(t *testing.T) {
	dir := t.TempDir()
	out, _, err := ExecIn("write-buit-"+t.Name(), dir, "write", `{"path":"buit.txt","content":""}`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "buit.txt"))
	if err != nil || len(raw) != 0 {
		t.Fatalf("havia de crear un fitxer buit: %q %v (%q)", raw, err, out)
	}
}
