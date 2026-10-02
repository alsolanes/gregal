package agent

// Diagnòstics per llenguatge més enllà del Go, el JSON i el YAML. La idea
// és la de l'LSP que opencode engega després de cada edició, però sense
// servidor ni dependències: només les eines que ja són a la màquina, i si
// no hi són, silenci. Una edició d'un .py en una màquina sense python no
// ha de dir «no he pogut comprovar res»: això és soroll que el model es
// menja a cada pas i no li serveix de res.
//
//	.py            sintaxi: python -m py_compile (python, py -3 o python3)
//	               tipus:   ruff (F821 nom no definit, F811 redefinició,
//	                        E9 errors d'E/S) o, si no, pyflakes filtrat
//	.ts .tsx ...   tipus:   tsc --noEmit del projecte, només si hi ha un
//	                        tsconfig.json pujant des del fitxer i un tsc
//	                        del projecte o al PATH
//	.sh .bash      sintaxi: sh -n / bash -n
//
// La sintaxi surt com a [sintaxi] i passa sempre; el que és de tipus surt
// com a [diagnòstic] i hooks.diag: syntax l'apaga, com el go vet.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Topalls de temps. La sintaxi és instantània; el ruff també (mil·lisegons).
// El tsc d'un projecte petit triga uns segons amb la cau freda; si en
// triga més de quinze, el projecte és prou gran perquè això faci més nosa
// que servei i es calla.
const (
	diagSintaxiTimeout = 5 * time.Second
	diagLintTimeout    = 5 * time.Second
	diagTSCTimeout     = 15 * time.Second
	diagProvaTimeout   = 3 * time.Second
)

// ---------------------------------------------------------------- Python

var (
	pythonUnCop sync.Once
	pythonExe   string
	pythonPre   []string // "-3" per al llançador py de Windows
)

// resolPython tria l'intèrpret una sola vegada per procés. A Windows el
// nom no diu res: python3 sol ser el fals de la Microsoft Store (a
// %LOCALAPPDATA%\Microsoft\WindowsApps), que no executa res i escriu
// «Python was not found» amb codi 9009; py és el llançador oficial i
// python és el de l'instal·lador. Per això no n'hi ha prou amb LookPath:
// cada candidat es prova amb un `-c pass` curt, i el primer que surt amb
// 0 és el bo. Fora de Windows, python3 primer (python pot ser el 2).
func resolPython() (string, []string, bool) {
	pythonUnCop.Do(func() {
		type cand struct {
			bin string
			pre []string
		}
		cands := []cand{{"python3", nil}, {"python", nil}}
		if runtime.GOOS == "windows" {
			cands = []cand{{"python", nil}, {"py", []string{"-3"}}, {"python3", nil}}
		}
		for _, c := range cands {
			exe, ok := trobaBinari(c.bin)
			if !ok {
				continue
			}
			args := append(append([]string{}, c.pre...), "-c", "pass")
			if r := execCurt(diagProvaTimeout, "", nil, exe, args...); r.acabat && r.err == nil {
				pythonExe, pythonPre = exe, c.pre
				return
			}
		}
	})
	return pythonExe, pythonPre, pythonExe != ""
}

// diagPython és la sintaxi: py_compile compila sense executar. Escriu
// __pycache__ al costat del fitxer, i per això la cau va a un directori
// temporal: no volem embrutar el repo de l'usuari a cada edició.
func diagPython(path string) string {
	exe, pre, ok := resolPython()
	if !ok {
		return ""
	}
	env := os.Environ()
	if cau, err := os.MkdirTemp("", "gregal-pycompile-*"); err == nil {
		defer os.RemoveAll(cau)
		env = append(env, "PYTHONPYCACHEPREFIX="+cau)
	}
	args := append(append([]string{}, pre...), "-m", "py_compile", path)
	r := execCurt(diagSintaxiTimeout, "", env, exe, args...)
	if !r.acabat || r.err == nil {
		return ""
	}
	return "el Python no compila (error de sintaxi):\n" + r.textOErr()
}

// diagPythonLint busca el que py_compile no veu i que trenca en temps
// d'execució: el nom no definit (la variable reanomenada a mitges, l'import
// oblidat) i la funció definida dues vegades. Res d'estil: F401 (import no
// usat) i companyia són opinió i farien soroll a cada edició. --isolated
// perquè la configuració del projecte no canviï la selecció ni faci petar
// el ruff amb una clau que aquesta versió no coneix.
func diagPythonLint(path string) string {
	dir, base := filepath.Dir(path), filepath.Base(path)
	if exe, ok := trobaBinari("ruff"); ok {
		r := execCurt(diagLintTimeout, dir, nil, exe, "check", "--isolated", "--no-cache",
			"--select", "F821,F811,E9", "--output-format", "concise", "--quiet", base)
		if !r.acabat || r.err == nil || strings.TrimSpace(r.out) == "" {
			return ""
		}
		return "[tipus] el ruff troba noms que petaran en executar:\n" + neteja(r.out)
	}
	if exe, ok := trobaBinari("pyflakes"); ok {
		r := execCurt(diagLintTimeout, dir, nil, exe, base)
		if !r.acabat || r.err == nil {
			return ""
		}
		// El pyflakes no té selecció: ens quedem amb les dues famílies que
		// equivalen a les del ruff de dalt.
		var linies []string
		for _, l := range strings.Split(r.out, "\n") {
			if strings.Contains(l, "undefined name") || strings.Contains(l, "redefinition of unused") {
				linies = append(linies, strings.TrimSpace(l))
			}
		}
		if len(linies) == 0 {
			return ""
		}
		return "[tipus] el pyflakes troba noms que petaran en executar:\n" + neteja(strings.Join(linies, "\n"))
	}
	return ""
}

// ---------------------------------------------------------------- Shell

// diagShell passa `bash -n` (o `sh -n` per a un .sh si no hi ha bash):
// només parseja, no executa res. A Windows cal anar amb compte amb quin
// bash es troba: el de WindowsApps i el de System32 són el del WSL, que
// no entén una ruta C:\… i arrencar-lo costa segons. Amb el del Git for
// Windows no hi ha problema.
func diagShell(path string) string {
	noms := []string{"bash", "sh"}
	for _, nom := range noms {
		exe, ok := trobaBinari(nom)
		if !ok || esShellWSL(exe) {
			continue
		}
		r := execCurt(diagSintaxiTimeout, filepath.Dir(path), nil, exe, "-n", filepath.Base(path))
		if !r.acabat || r.err == nil {
			return ""
		}
		return "l'script de shell no parseja (" + nom + " -n):\n" + r.textOErr()
	}
	return ""
}

func esShellWSL(exe string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	l := strings.ToLower(exe)
	return strings.Contains(l, `\windowsapps\`) || strings.Contains(l, `\system32\`)
}

// ---------------------------------------------------------------- TypeScript

// diagTypeScript és l'únic cas que mira el projecte sencer: el tsc no sap
// comprovar un fitxer solt respectant el tsconfig (paths, jsx, strict), i
// sense tsconfig els errors serien de configuració, no de l'edició. Per
// això: sense tsconfig.json pujant des del fitxer, o sense tsc, silenci.
// Dels errors, només els del fitxer editat; dels altres, el recompte, que
// diu si la refactorització ha deixat coses penjades sense omplir el
// context amb la llista.
func diagTypeScript(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	arrel, ok := trobaTSConfig(filepath.Dir(abs))
	if !ok {
		return ""
	}
	exe, pre, ok := trobaTSC(arrel)
	if !ok {
		return ""
	}
	args := append(append([]string{}, pre...), "--noEmit", "--pretty", "false", "-p", arrel)
	r := execCurt(diagTSCTimeout, arrel, verifyEnv(), exe, args...)
	if !r.acabat || r.err == nil {
		return ""
	}
	propis, altres := filtraTSC(r.out, arrel, abs)
	if len(propis) == 0 {
		return ""
	}
	msg := "[tipus] el tsc troba errors en aquest fitxer:\n" + neteja(strings.Join(propis, "\n"))
	if altres > 0 {
		msg += "\n(i " + strconv.Itoa(altres) + " errors en altres fitxers del projecte)"
	}
	return msg
}

// trobaTSConfig puja des de dir fins a trobar un tsconfig.json. S'atura a
// l'arrel del repo (.git) o a dotze nivells: un tsconfig de fora del repo
// no és d'aquest projecte.
func trobaTSConfig(dir string) (string, bool) {
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return dir, true
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", false
		}
		pare := filepath.Dir(dir)
		if pare == dir {
			return "", false
		}
		dir = pare
	}
	return "", false
}

// trobaTSC prefereix el typescript del projecte (la versió que el projecte
// ha fixat) al global. Primer node + node_modules/typescript/bin/tsc: és
// JavaScript pla i va igual a tot arreu, sense el .cmd de Windows que, en
// matar-lo pel timeout, deixa el node fill viu. Després node_modules/.bin
// i, al final, el tsc del PATH.
func trobaTSC(arrel string) (string, []string, bool) {
	node, teNode := trobaBinari("node")
	dir := arrel
	for i := 0; i < 12; i++ {
		js := filepath.Join(dir, "node_modules", "typescript", "bin", "tsc")
		if st, err := os.Stat(js); teNode && err == nil && !st.IsDir() {
			return node, []string{js}, true
		}
		bin := filepath.Join(dir, "node_modules", ".bin", "tsc")
		if runtime.GOOS == "windows" {
			bin += ".cmd"
		}
		if st, err := os.Stat(bin); err == nil && !st.IsDir() {
			return bin, nil, true
		}
		pare := filepath.Dir(dir)
		if pare == dir {
			break
		}
		dir = pare
	}
	if exe, ok := trobaBinari("tsc"); ok {
		return exe, nil, true
	}
	return "", nil, false
}

// «src/a.ts(3,5): error TS2304: Cannot find name 'x'.» amb --pretty false.
var reErrorTSC = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): error TS\d+:`)

// filtraTSC separa els errors del fitxer editat dels de la resta. El tsc
// escriu les rutes relatives al directori on corre (arrel); a Windows, sense
// distingir majúscules, que la mateixa ruta pot arribar com C:\ o c:\.
// Les línies de continuació (sagnades, sense capçalera) es descarten: la
// capçalera ja diu on i què.
func filtraTSC(out, arrel, abs string) ([]string, int) {
	var propis []string
	altres := 0
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimRight(l, "\r")
		m := reErrorTSC.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		p := filepath.FromSlash(m[1])
		if !filepath.IsAbs(p) {
			p = filepath.Join(arrel, p)
		}
		if mateixaRuta(p, abs) {
			propis = append(propis, strings.TrimSpace(l))
		} else {
			altres++
		}
	}
	return propis, altres
}

func mateixaRuta(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
