package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func teNode() bool   { _, ok := trobaBinari("node"); return ok }
func tePython() bool { _, _, ok := resolPython(); return ok }
func teShell() bool {
	for _, n := range []string{"bash", "sh"} {
		if p, ok := trobaBinari(n); ok && !esShellWSL(p) {
			return true
		}
	}
	return false
}

// Un fitxer bo calla i un de trencat avisa amb [sintaxi], per a cada
// llenguatge que es comprova amb una eina externa. Si l'eina no hi és, el
// cas se salta: el que es prova és la integració, no la màquina.
func TestDiagnosticaLlenguatgesExterns(t *testing.T) {
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })
	casos := []struct {
		nom, cos string
		cal      func() bool
		conte    string // "" = ha de callar
	}{
		{"bo.py", "def f(a):\n    return a + 1\n", tePython, ""},
		{"mal.py", "def f(:\n    pass\n", tePython, "Python"},
		{"bo.js", "function f(a) { return a + 1 }\n", teNode, ""},
		{"mal.js", "function f( {\n", teNode, "JavaScript"},
		{"bo.mjs", "export const a = 1\n", teNode, ""},
		{"mal.mjs", "export const = 1\n", teNode, "JavaScript"},
		{"mal.cjs", "module.exports = {\n", teNode, "JavaScript"},
		{"bo.sh", "#!/bin/sh\nif [ -n \"$1\" ]; then\n  echo \"$1\"\nfi\n", teShell, ""},
		{"mal.sh", "#!/bin/sh\nif true; then\n  echo hola\n", teShell, "shell"},
		{"mal.bash", "for i in 1 2; do\n  echo $i\n", teShell, "shell"},
	}
	for _, c := range casos {
		t.Run(c.nom, func(t *testing.T) {
			if !c.cal() {
				t.Skip("sense l'eina en aquesta màquina")
			}
			d := Diagnostica(escriu(t, c.nom, c.cos))
			if c.conte == "" {
				if d != "" {
					t.Fatalf("fitxer bo i diu: %q", d)
				}
				return
			}
			if !strings.HasPrefix(d, "[sintaxi] ") || !strings.Contains(d, c.conte) {
				t.Fatalf("trencat: vol [sintaxi] i %q, diu: %q", c.conte, d)
			}
		})
	}
}

// py_compile no pot deixar __pycache__ al directori de l'usuari.
func TestDiagPythonNoEmbrutaElDirectori(t *testing.T) {
	if !tePython() {
		t.Skip("sense python")
	}
	p := escriu(t, "net.py", "x = 1\n")
	Diagnostica(p)
	if _, err := os.Stat(filepath.Join(filepath.Dir(p), "__pycache__")); err == nil {
		t.Fatal("ha deixat __pycache__ al costat del fitxer")
	}
}

// L'intèrpret triat ha d'executar de debò (a Windows, python3 pot ser el
// fals de la Store): si resolPython el dona, un `-c pass` ha d'anar bé.
func TestResolPythonExecutaDeDebo(t *testing.T) {
	exe, pre, ok := resolPython()
	if !ok {
		t.Skip("sense python")
	}
	args := append(append([]string{}, pre...), "-c", "import sys; print(sys.version_info[0])")
	r := execCurt(diagProvaTimeout, "", nil, exe, args...)
	if !r.acabat || r.err != nil || strings.TrimSpace(r.out) != "3" {
		t.Fatalf("%s %v: %+v", exe, pre, r)
	}
}

// El nom no definit (el que py_compile no veu) surt com a [diagnòstic] si
// hi ha ruff o pyflakes; en mode syntax, no.
func TestDiagPythonNomNoDefinit(t *testing.T) {
	_, ruff := trobaBinari("ruff")
	_, pyflakes := trobaBinari("pyflakes")
	if !tePython() || (!ruff && !pyflakes) {
		t.Skip("sense ruff ni pyflakes")
	}
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })
	p := escriu(t, "nom.py", "import os\n\ndef f():\n    return valor_que_no_existeix\n")
	d := Diagnostica(p)
	if !strings.HasPrefix(d, "[diagnòstic] ") || !strings.Contains(d, "valor_que_no_existeix") {
		t.Fatalf("nom no definit: %q", d)
	}
	// L'import no usat és estil: no ha de sortir.
	if strings.Contains(d, "F401") || strings.Contains(d, "imported but unused") {
		t.Fatalf("soroll d'estil: %q", d)
	}
	SetDiagMode("syntax")
	if d := Diagnostica(p); d != "" {
		t.Fatalf("en mode syntax no hi ha lint: %q", d)
	}
}

// Un projecte TypeScript amb un tsc fals (un script de node a
// node_modules/typescript/bin/tsc, com el de debò): es prova tota la
// cadena —tsconfig, resolució del tsc, filtre al fitxer editat, recompte
// dels altres— sense haver d'instal·lar el typescript.
func projecteTSFals(t *testing.T) string {
	t.Helper()
	if !teNode() {
		t.Skip("sense node")
	}
	arrel := t.TempDir()
	os.WriteFile(filepath.Join(arrel, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`), 0o644)
	bin := filepath.Join(arrel, "node_modules", "typescript", "bin")
	os.MkdirAll(bin, 0o755)
	fals := `if (!process.argv.includes("--noEmit")) { console.log("falta --noEmit"); process.exit(9) }
console.log("src/a.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.")
console.log("  The expected type comes from here.")
console.log("src/b.ts(2,1): error TS2304: Cannot find name 'y'.")
process.exit(2)
`
	os.WriteFile(filepath.Join(bin, "tsc"), []byte(fals), 0o644)
	os.MkdirAll(filepath.Join(arrel, "src"), 0o755)
	for _, f := range []string{"a.ts", "c.ts"} {
		os.WriteFile(filepath.Join(arrel, "src", f), []byte("export const x = 1\n"), 0o644)
	}
	return arrel
}

func TestDiagTypeScriptFiltraAlFitxerEditat(t *testing.T) {
	arrel := projecteTSFals(t)
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })

	d := Diagnostica(filepath.Join(arrel, "src", "a.ts"))
	if !strings.HasPrefix(d, "[diagnòstic] ") || !strings.Contains(d, "TS2322") {
		t.Fatalf("a.ts: %q", d)
	}
	if strings.Contains(d, "TS2304") || !strings.Contains(d, "1 errors en altres fitxers") {
		t.Fatalf("a.ts no ha de llistar els errors de b.ts, només comptar-los: %q", d)
	}
	// Un fitxer sense errors propis calla encara que el projecte en tingui.
	if d := Diagnostica(filepath.Join(arrel, "src", "c.ts")); d != "" {
		t.Fatalf("c.ts no té errors: %q", d)
	}
	SetDiagMode("syntax")
	if d := Diagnostica(filepath.Join(arrel, "src", "a.ts")); d != "" {
		t.Fatalf("en mode syntax no hi ha tsc: %q", d)
	}
}

// Sense tsconfig.json (o amb el .git abans del tsconfig) no es passa el
// tsc: els errors serien de configuració, no de l'edició.
func TestDiagTypeScriptSenseTSConfigCalla(t *testing.T) {
	arrel := projecteTSFals(t)
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })
	os.Remove(filepath.Join(arrel, "tsconfig.json"))
	if d := Diagnostica(filepath.Join(arrel, "src", "a.ts")); d != "" {
		t.Fatalf("sense tsconfig: %q", d)
	}
	// tsconfig a fora del repo: el .git talla la pujada.
	fora := t.TempDir()
	os.WriteFile(filepath.Join(fora, "tsconfig.json"), []byte(`{}`), 0o644)
	repo := filepath.Join(fora, "repo")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "src"), 0o755)
	if _, ok := trobaTSConfig(filepath.Join(repo, "src")); ok {
		t.Fatal("el tsconfig de fora del repo no és d'aquest projecte")
	}
}

func TestFiltraTSC(t *testing.T) {
	arrel := t.TempDir()
	abs := filepath.Join(arrel, "src", "a.ts")
	out := strings.Join([]string{
		"src/a.ts(3,5): error TS2304: Cannot find name 'x'.",
		"  continuació que es descarta",
		abs + "(4,1): error TS1005: ';' expected.",
		"src/altre.ts(1,1): error TS2304: Cannot find name 'z'.",
		"src/a.tsx(1,1): error TS2304: un altre fitxer amb el mateix prefix.",
		"error TS5058: The specified path does not exist.",
	}, "\r\n")
	propis, altres := filtraTSC(out, arrel, abs)
	if len(propis) != 2 || altres != 2 {
		t.Fatalf("propis=%q altres=%d", propis, altres)
	}
}

// El topall és de debò: una eina que es penja no pot aturar el torn.
func TestExecCurtRespectaElTopall(t *testing.T) {
	node, ok := trobaBinari("node")
	if !ok {
		t.Skip("sense node")
	}
	t0 := time.Now()
	r := execCurt(300*time.Millisecond, "", nil, node, "-e", "setTimeout(() => {}, 20000)")
	if r.acabat {
		t.Fatalf("hauria d'haver saltat el topall: %+v", r)
	}
	if dt := time.Since(t0); dt > 3*time.Second {
		t.Fatalf("ha trigat %v amb un topall de 300ms", dt)
	}
	// Un binari que no existeix tampoc compta com a acabat.
	if r := execCurt(time.Second, "", nil, filepath.Join(t.TempDir(), "no-hi-es")); r.acabat {
		t.Fatalf("binari inexistent: %+v", r)
	}
}

// Mai no es fa servir el bash del WSL: no entén rutes C:\ i arrencar-lo
// triga segons.
func TestEsShellWSL(t *testing.T) {
	if p, err := exec.LookPath("bash"); err == nil && esShellWSL(p) {
		t.Logf("el primer bash del PATH és el del WSL (%s): es salta", p)
	}
	if esShellWSL(`C:\Program Files\Git\usr\bin\bash.exe`) {
		t.Fatal("el bash del Git for Windows és bo")
	}
}
