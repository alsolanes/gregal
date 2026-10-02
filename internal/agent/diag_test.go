package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func escriu(t *testing.T, nom, cos string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), nom)
	if err := os.WriteFile(p, []byte(cos), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Un fitxer correcte no ha de dir res: callar és el cas normal i qualsevol
// soroll aquí se'l menja el context de cada edició.
func TestDiagnosticaCallaSiTotVaBe(t *testing.T) {
	casos := map[string]string{
		"bo.go":   "package calc\n\nfunc Suma(a, b int) int { return a + b }\n",
		"bo.json": `{"a": 1, "b": [2, 3]}`,
		"bo.yaml": "arrel:\n  clau: valor\n  llista:\n    - un\n",
		"bo.yml":  "a: 1\n",
		// Una extensió que no sabem comprovar tampoc no diu res.
		"bo.rs":  "fn main() { let x = ; }",
		"bo.txt": "el que sigui",
	}
	for nom, cos := range casos {
		if d := Diagnostica(escriu(t, nom, cos)); d != "" {
			t.Errorf("%s hauria de callar, diu: %q", nom, d)
		}
	}
}

// El cas que això ataca: l'agent deixa el fitxer trencat i l'eina responia
// «escrit, N bytes» i tan amples.
func TestDiagnosticaEnganxaElsTrencats(t *testing.T) {
	casos := map[string]struct{ cos, conte string }{
		"mal.go":   {"package calc\n\nfunc Suma(a, b int) int { return a + b\n", "sintaxi"},
		"mal.json": {"{\"a\": 1,\n \"b\": 2,,}", "JSON"},
		"mal.yaml": {"arrel:\n  clau: valor\n   mal: indentat\n", "YAML"},
	}
	for nom, c := range casos {
		d := Diagnostica(escriu(t, nom, c.cos))
		if d == "" {
			t.Errorf("%s està trencat i no ho diu", nom)
			continue
		}
		if !strings.HasPrefix(d, "[sintaxi] ") {
			t.Errorf("%s: falta la marca: %q", nom, d)
		}
		if !strings.Contains(d, c.conte) {
			t.Errorf("%s: hauria de dir %q: %q", nom, c.conte, d)
		}
	}
}

// Un offset de bytes no diu res a ningú; la línia i la columna sí.
func TestDiagnosticaJSONDiuLiniaIColumna(t *testing.T) {
	d := Diagnostica(escriu(t, "x.json", "{\n  \"a\": 1,\n  \"b\": 2,,\n}\n"))
	if !strings.Contains(d, "línia 3") {
		t.Fatalf("hauria de dir la línia: %q", d)
	}
	if !strings.Contains(d, "columna") {
		t.Fatalf("hauria de dir la columna: %q", d)
	}
}

func TestLiniaCol(t *testing.T) {
	raw := []byte("un\ndos\ntres")
	casos := []struct{ off, l, c int }{
		{0, 1, 1}, {1, 1, 2}, {3, 2, 1}, {7, 3, 1},
		{999, 3, 5}, // fora de rang: s'enganxa al final, no peta
	}
	for _, x := range casos {
		l, c := liniaCol(raw, x.off)
		if l != x.l || c != x.c {
			t.Errorf("liniaCol(%d) = %d,%d; volia %d,%d", x.off, l, c, x.l, x.c)
		}
	}
}

// Un error de sintaxi en cascada en pot escopir cinquanta; la primera ja
// diu on és i la resta omple el context per no res.
func TestNetejaRetallaLaCascada(t *testing.T) {
	s := neteja("un\n\ndos\ntres\nquatre\ncinc")
	if strings.Count(s, "\n") != 3 {
		t.Fatalf("volia 4 línies: %q", s)
	}
	if !strings.Contains(s, "i 2 errors més") {
		t.Fatalf("ha de dir quants en queden: %q", s)
	}
}

// El resultat d'una edició ha de dur la sintaxi, i davant del hook: si el
// fitxer ha quedat trencat, és el que el model ha de llegir primer.
func TestApendixEdicio(t *testing.T) {
	SetPostEditHook("")
	t.Cleanup(func() { SetPostEditHook("") })
	bo := escriu(t, "bo.json", `{"a":1}`)
	if a := apendixEdicio(bo); a != "" {
		t.Fatalf("amb tot bé i sense hook no ha de dir res: %q", a)
	}
	mal := escriu(t, "mal.json", `{"a":}`)
	a := apendixEdicio(mal)
	if !strings.HasPrefix(a, "\n[sintaxi] ") {
		t.Fatalf("l'apèndix ha de començar amb la sintaxi: %q", a)
	}
}

// El camí de debò per a .py i .go: el que torna l'eina write ha d'avisar
// si el fitxer ha quedat trencat (punt únic: apendixEdicio → Diagnostica).
func TestExecWriteAvisaDeLaSintaxiPyGo(t *testing.T) {
	SetPostEditHook("")
	t.Cleanup(func() { SetPostEditHook("") })
	dir := t.TempDir()
	casos := map[string]string{
		"trencat.py": "def f(:\n    pass\n",
		"trencat.go": "package calc\n\nfunc f( {\n",
		"trencat.js": "function f( {\n",
	}
	// Binari extern necessari per extensió ("": en procés, sempre hi és).
	// diagExtern calla si el binari no hi és: el test ha de saltar el cas,
	// no fallar, en màquines sense node/python.
	cal := map[string]func() bool{"trencat.py": tePython, "trencat.js": teNode}
	for nom, cos := range casos {
		nom, cos := nom, cos
		t.Run(nom, func(t *testing.T) {
			if c := cal[nom]; c != nil && !c() {
				t.Skipf("sense l'eina: salto %s", nom)
			}
			p := filepath.Join(dir, nom)
			args, _ := json.Marshal(map[string]string{"path": p, "content": cos})
			out, _, err := Exec("write", string(args))
			if err != nil {
				t.Fatal(err)
			}
			// El marcador amb claudàtors: el path de t.TempDir() du el nom del
			// test i un Contains sense claudàtors seria vacu.
			if !strings.Contains(strings.ToUpper(out), "[SINTAXI]") {
				t.Errorf("%s trencat i no avisa: %q", nom, out)
			}
		})
	}
}

// El vet caça l'import que falta (el cas T6: strconv usat sense importar,
// el model ho veu al go test tres passos tard o no ho veu).
func TestDiagGoVetImportQueFalta(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sense toolchain go")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module vett\n\ngo 1.22\n"), 0o644)
	p := filepath.Join(dir, "main.go")
	os.WriteFile(p, []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(strconv.Atoi(\"3\")) }\n"), 0o644)
	out := Diagnostica(p)
	if !strings.Contains(out, "strconv") {
		t.Fatalf("havia de mencionar strconv: %q", out)
	}
	if !strings.Contains(out, "[tipus]") {
		t.Fatalf("havia de marcar-ho com a tipus, no sintaxi: %q", out)
	}
}

// El vet no fa soroll dels altres fitxers: en una refactorització a mitges,
// o amb tests que fallen per disseny, només parla del fitxer editat.
func TestDiagGoVetFiltraAltresFitxers(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sense toolchain go")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module vetf\n\ngo 1.22\n"), 0o644)
	bo := filepath.Join(dir, "bo.go")
	os.WriteFile(bo, []byte("package main\n\nfunc Bo() int { return 1 }\n"), 0o644)
	dolent := filepath.Join(dir, "dolent.go")
	os.WriteFile(dolent, []byte("package main\n\nfunc Dolent() int { return noExisteix }\n"), 0o644)
	if out := Diagnostica(bo); out != "" {
		t.Fatalf("el fitxer bo ha de callar encara que el paquet falli: %q", out)
	}
	if out := Diagnostica(dolent); !strings.Contains(out, "noExisteix") {
		t.Fatalf("el fitxer dolent sí que ha d'avisar: %q", out)
	}
}

// Sense go.mod proper el vet calla (fora de mòdul dona errors espuris).
func TestDiagGoVetSenseGoModCalla(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sense toolchain go")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "sol.go")
	os.WriteFile(p, []byte("package main\n\nfunc Sol() int { return noExisteix }\n"), 0o644)
	if out := Diagnostica(p); out != "" {
		t.Fatalf("sense go.mod ha de callar: %q", out)
	}
}

// trobaBinari ha de trobar node al runtime d'Hermes encara que no sigui al
// PATH (cas T7: el check de .js no corria mai). I un binari inexistent no
// existeix enlloc.
func TestTrobaBinariNode(t *testing.T) {
	home, _ := os.UserHomeDir()
	esperat := filepath.Join(home, ".hermes", "node", "bin", "node")
	if !potExecutar(esperat) {
		t.Skip("sense node al runtime d'Hermes en aquesta màquina")
	}
	p, ok := trobaBinari("node")
	if !ok {
		t.Fatal("node hi és però trobaBinari no el troba")
	}
	if p != esperat {
		if _, err := exec.LookPath("node"); err != nil {
			t.Fatalf("trobaBinari torna %q, volia %q", p, esperat)
		}
	}
	if _, ok := trobaBinari("binari-que-no-existeix-xyz"); ok {
		t.Fatal("un binari inexistent no es pot trobar")
	}
}

// Un .js trencat ha d'avisar (amb node del runtime si cal).
func TestDiagnosticaJSTrencat(t *testing.T) {
	if _, ok := trobaBinari("node"); !ok {
		t.Skip("sense node enlloc")
	}
	p := filepath.Join(t.TempDir(), "trencat.js")
	os.WriteFile(p, []byte("function f( {\n"), 0o644)
	if out := Diagnostica(p); !strings.Contains(strings.ToUpper(out), "JAVASCRIPT") {
		t.Fatalf("js trencat i no avisa: %q", out)
	}
}

// Un fitxer gros i correcte no pot costar car: això corre després de CADA
// edició.
func BenchmarkDiagnosticaGo(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("package gran\n")
	for i := 0; i < 400; i++ {
		sb.WriteString("func F" + string(rune('A'+i%26)) + "() int { return 1 }\n")
	}
	p := filepath.Join(b.TempDir(), "gran.go")
	os.WriteFile(p, []byte(sb.String()), 0o644)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Diagnostica(p)
	}
}

// El camí de debò: el que torna l'eina write. Abans deia «escrit, N bytes»
// encara que el fitxer hagués quedat inservible.
func TestExecWriteAvisaDeLaSintaxi(t *testing.T) {
	SetPostEditHook("")
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	args, _ := json.Marshal(map[string]string{"path": p, "content": "{\n  \"a\": 1,,\n}"})
	out, _, err := Exec("write", string(args))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "escrit") {
		t.Fatalf("hauria de dir que l'ha escrit: %q", out)
	}
	if !strings.Contains(out, "[sintaxi]") {
		t.Fatalf("l'eina ha d'avisar que ha quedat trencat: %q", out)
	}
	// I amb un de bo, la sortida es queda neta.
	args, _ = json.Marshal(map[string]string{"path": filepath.Join(dir, "ok.json"), "content": `{"a":1}`})
	out, _, _ = Exec("write", string(args))
	if strings.Contains(out, "[sintaxi]") {
		t.Fatalf("un fitxer correcte no ha de dir res: %q", out)
	}
}
