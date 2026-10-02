package agent

// Diagnòstics després d'editar que van més enllà de la sintaxi: el
// compilador o l'intèrpret del llenguatge, si és a la màquina, sobre el
// fitxer (o el paquet) que s'acaba de tocar. El resultat s'enganxa al
// retorn de write/edit/patch, així el model veu «undefined: Timeout» al
// mateix pas que ha fet el canvi, no tres passos després quan es
// decideix a passar els tests. És el que fan Claude Code i opencode amb
// l'LSP; aquí, sense servidor de llenguatge, amb les eines de línia
// d'ordres que ja hi ha:
//
//	.go            go vet del paquet (diagGoVet, a diag.go)
//	.py            ruff o pyflakes, noms no definits (diag_lang.go)
//	.ts .tsx ...   tsc --noEmit del projecte (diag_lang.go)
//
// La sintaxi d'aquests llenguatges (py_compile, node --check, bash -n) ja
// s'ha mirat abans, a Diagnostica, i surt com a [sintaxi].
//
// Cada comprovació té un topall de temps curt: si l'eina no és a la
// màquina o triga massa, no es diu res.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DiagExecTimeout és el màxim per comprovació externa. go vet d'un paquet
// amb la cache calenta triga un o dos segons; si en triga més de deu, no
// val la pena fer esperar el torn.
const DiagExecTimeout = 10 * time.Second

// diagExecDisabled desactiva les comprovacions externes (tests ràpids).
var diagExecDisabled = false

// diagExterna executa la comprovació de tipus del llenguatge i torna el
// text d'error net, o "" si tot va bé o no es pot comprovar.
func diagExterna(path string) string {
	if diagExecDisabled {
		return ""
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return diagPythonLint(path)
	case ".ts", ".tsx", ".mts", ".cts":
		return diagTypeScript(path)
	}
	// El Go ja passa per diagGoVet (diag.go), filtrat al fitxer editat.
	return ""
}

// resultatExec és el que queda d'una execució curta. acabat és fals si no
// s'ha pogut arrencar o si ha saltat el topall: en tots dos casos, qui
// crida calla.
type resultatExec struct {
	out    string
	err    error
	acabat bool
}

// textOErr torna la sortida neta o, si l'eina no ha escrit res, l'error
// del procés (el codi de sortida, almenys).
func (r resultatExec) textOErr() string {
	if t := strings.TrimSpace(neteja(r.out)); t != "" {
		return t
	}
	if r.err != nil {
		return strings.TrimSpace(r.err.Error())
	}
	return ""
}

// execCurt és l'única manera d'executar una eina de diagnòstic: argv
// directe (sense shell), sortida combinada, topall de temps i WaitDelay.
// El WaitDelay és el que fa que el topall sigui de debò: sense ell, si
// l'eina és un .cmd o un script que engega un fill (tsc → node), el
// context mata el pare però el fill es queda amb la pipe oberta i el
// CombinedOutput espera que acabi pel seu compte. env nil hereta l'entorn.
func execCurt(timeout time.Duration, dir string, env []string, exe string, args ...string) resultatExec {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, exe, args...)
	c.Dir = dir
	c.Env = env
	c.WaitDelay = time.Second
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := c.Run()
	if ctx.Err() != nil {
		return resultatExec{} // massa lent: no bloquegem el torn per això
	}
	if _, esSortida := err.(*exec.ExitError); err != nil && !esSortida {
		return resultatExec{} // no ha arrencat (binari fals, permisos)
	}
	return resultatExec{out: out.String(), err: err, acabat: true}
}

// dinsModulGo puja directoris buscant un go.mod.
func dinsModulGo(dir string) bool {
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return true
		}
		pare := filepath.Dir(dir)
		if pare == dir {
			return false
		}
		dir = pare
	}
	return false
}

func retallaLinies(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n… (" + itoa(len(lines)-n) + " línies més)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
