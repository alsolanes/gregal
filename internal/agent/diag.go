package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// Comprovació de sintaxi després d'escriure.
//
// El mode de fallada que això ataca: l'agent escriu un fitxer, el deixa
// sintàcticament trencat, i no se n'assabenta fins tres passos més tard —o
// no se n'assabenta— perquè l'eina write torna «escrit, 412 bytes» i tan
// amples. Un JSON amb una coma de més al config, un YAML mal indentat, una
// clau que falta a un .go: tot això es detecta en un moment i, si es diu de
// seguida, el model ho arregla al pas següent.
//
// A posta en procés quan es pot (go/parser, encoding/json, yaml.v3):
// és instantani, funciona igual a Windows, i no depèn de tenir res
// instal·lat. L'excepció és el Go: després de la sintaxi, un `go vet` al
// paquet (dècimes de segon amb la cau calenta) caça l'import que falta i
// la variable no usada, filtrat al fitxer editat perquè la
// refactorització a mitges no faci soroll. Per a la resta ja hi ha
// hooks.post_edit, que és on posar-hi el teu linter.
//   - Externs (py_compile, node --check, bash -n): només parsegen, amb
//     un topall de pocs segons, i callen si el binari no hi és. Són
//     auxiliars, no la via principal (diag_lang.go).
const MaxDiagChars = 600

// Diagnostica mira si el fitxer segueix sent vàlid. Torna "" si tot bé o
// si l'extensió no és de les que sap comprovar: callar és el cas normal.
//
// Revisió del criteri de dalt («a posta NOMÉS sintaxi»): amb les lectures
// en paral·lel i el pressupost de passos tou, el que més passos crema
// avui és descobrir un error de tipus tres passos després d'haver-lo
// escrit. Per això, després de la sintaxi, si hooks.diag no diu
// "syntax", es passa també el compilador o l'intèrpret del llenguatge
// (diag_exec.go) amb un topall de temps curt. Els errors intermedis d'una
// refactorització de tres fitxers hi seran, sí, però el model sap què
// està fent i els resol al pas següent en comptes d'al final.
func Diagnostica(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "" // si no es pot llegir, ja ho dirà qui el necessiti
	}
	var sintaxi string
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		sintaxi = diagGo(path, raw)
	case ".json":
		return capDiag(diagJSON(raw))
	case ".yaml", ".yml":
		return capDiag(diagYAML(raw))
	case ".py":
		// Sintaxi aquí; si és neta, el ruff/pyflakes passa per diagExterna.
		sintaxi = diagPython(path)
	case ".js", ".mjs", ".cjs":
		return capDiag(diagExtern("el JavaScript no compila (error de sintaxi)", "node", "--check", path))
	case ".sh", ".bash":
		sintaxi = diagShell(path)
	}
	if sintaxi != "" {
		return capDiag(sintaxi)
	}
	if getDiagMode() == "syntax" {
		return ""
	}
	if ext := diagExterna(path); ext != "" {
		return capDiagN(ext, 1200)
	}
	return ""
}

// diagMode és hooks.diag: "" o "full" (defecte) | "syntax".
var diagMode atomic.Value // string

// SetDiagMode fixa el mode de diagnòstic ("syntax" o "full").
func SetDiagMode(mode string) { diagMode.Store(strings.ToLower(strings.TrimSpace(mode))) }

func getDiagMode() string {
	v, _ := diagMode.Load().(string)
	return v
}

func capDiagN(s string, n int) string {
	if s == "" {
		return ""
	}
	if len(s) > n {
		s = s[:n] + "…"
	}
	return "[diagnòstic] " + s
}

func capDiag(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > MaxDiagChars {
		s = s[:MaxDiagChars] + "…"
	}
	return "[sintaxi] " + s
}

func diagGo(path string, raw []byte) string {
	fs := token.NewFileSet()
	// ParseComments i tot el fitxer: volem l'error de sintaxi, no una
	// anàlisi.
	if _, err := parser.ParseFile(fs, path, raw, parser.ParseComments|parser.AllErrors); err != nil {
		return "el fitxer no compila (error de sintaxi):\n" + neteja(err.Error())
	}
	// Sintaxi neta: un `go vet` al paquet caça l'error típic d'escriptura
	// (import que falta, variable no usada) en dècimes de segon amb la
	// cau calenta (mesurat: 0,09 s). Només es reporten els errors que
	// mencionen el fitxer editat: en una refactorització de tres fitxers
	// els errors intermedis dels altres són soroll, i els tests que fallen
	// per disseny (TDD) no són culpa d'aquesta edició.
	if getDiagMode() == "syntax" {
		return ""
	}
	return diagGoVet(path)
}

// diagGoVet passa `go vet` al directori del fitxer i en torna només els
// errors del fitxer editat. Sense toolchain, sense go.mod proper o amb
// timeout, calla: és un avís, no un error.
func diagGoVet(path string) string {
	if _, err := exec.LookPath("go"); err != nil {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	if !teGoMod(dir) {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "go", "vet", ".")
	c.Dir = dir
	raw, err := c.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return ""
	}
	if err == nil {
		return ""
	}
	base := filepath.Base(abs)
	linies := []string{}
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.Contains(l, base) {
			linies = append(linies, strings.TrimSpace(l))
		}
	}
	if len(linies) == 0 {
		return ""
	}
	return "[tipus] el paquet no passa el vet per aquest fitxer:\n" + neteja(strings.Join(linies, "\n"))
}

// teGoMod diu si dir o algun avantpassat té go.mod (el vet fora de mòdul
// dona errors espuris de mode GOPATH).
func teGoMod(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return true
		}
		pare := filepath.Dir(dir)
		if pare == dir {
			return false
		}
		dir = pare
	}
}

func diagJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		msg := err.Error()
		// L'offset sol no diu res: amb la línia i la columna s'hi va de dret.
		if se, ok := err.(*json.SyntaxError); ok {
			l, c := liniaCol(raw, int(se.Offset))
			msg = fmt.Sprintf("%s (línia %d, columna %d)", se.Error(), l, c)
		}
		return "el JSON no es pot llegir: " + msg
	}
	return ""
}

func diagYAML(raw []byte) string {
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return "el YAML no es pot llegir: " + neteja(err.Error())
	}
	return ""
}

// diagExtern comprova la sintaxi amb un binari extern (node): només
// parseja, no reescriu res. El binari es busca al PATH i als toolchains
// coneguts (verifyEnv: node viu a ~/.hermes/node/bin, fora del PATH); si
// no hi és, calla. Sense shell: argv directe + topall (execCurt). No fa
// fallar mai l'eina: un check que falla és un avís, no un error.
func diagExtern(descripcio, bin string, args ...string) string {
	binari, ok := trobaBinari(bin)
	if !ok {
		return ""
	}
	r := execCurt(diagSintaxiTimeout, "", verifyEnv(), binari, args...)
	if !r.acabat || r.err == nil {
		return ""
	}
	return descripcio + ":\n" + r.textOErr()
}

// trobaBinari busca bin al PATH i als directoris de toolchains coneguts.
// Torna la ruta absoluta per cridar-lo sense dependre del PATH heretat.
func trobaBinari(bin string) (string, bool) {
	if p, err := exec.LookPath(bin); err == nil {
		return p, true
	}
	home, _ := os.UserHomeDir()
	for _, d := range []string{
		filepath.Join(home, ".hermes", "node", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "sdk", "go", "bin"),
	} {
		if p := filepath.Join(d, bin); potExecutar(p) {
			return p, true
		}
	}
	return "", false
}

func potExecutar(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// liniaCol converteix un offset de bytes en línia i columna (1-based).
func liniaCol(raw []byte, off int) (int, int) {
	if off > len(raw) {
		off = len(raw)
	}
	linia, ultimSalt := 1, -1
	for i := 0; i < off; i++ {
		if raw[i] == '\n' {
			linia++
			ultimSalt = i
		}
	}
	return linia, off - ultimSalt
}

// neteja treu els salts de línia repetits i deixa les tres primeres
// línies: un error de sintaxi en cascada en pot escopir cinquanta i la
// primera ja diu on és.
func neteja(s string) string {
	parts := []string{}
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			parts = append(parts, l)
		}
	}
	if len(parts) > 3 {
		parts = append(parts[:3], fmt.Sprintf("… i %d errors més", len(parts)-3))
	}
	return strings.Join(parts, "\n")
}
