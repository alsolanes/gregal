// Package shell tria l'intèrpret d'ordres de la màquina. Un sol lloc per a
// l'eina bash de l'agent i per als processos en segon pla del terminal: abans
// cadascun ho decidia pel seu compte (l'eina amb cmd a Windows, el terminal
// sempre amb sh) i el portable de Windows moria amb «"sh": executable file
// not found in %PATH%» en engegar qualsevol procés.
package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// gitBashCandidates són on Git for Windows deixa el seu sh.exe. És el que
// tenen la majoria de màquines Windows de desenvolupament, i entén les
// ordres que la gent escriu (&&, pipes, `npm run dev`).
var gitBashCandidates = []string{
	`C:\Program Files\Git\bin\sh.exe`,
	`C:\Program Files\Git\usr\bin\sh.exe`,
	`C:\Program Files (x86)\Git\bin\sh.exe`,
}

var (
	once     sync.Once
	resolved string
	posix    bool
)

// resolve troba l'intèrpret un sol cop: sh al PATH; a Windows, el sh del
// Git si hi és; si no, cmd. Es pot forçar amb GREGAL_SHELL (ruta d'un sh
// o "cmd").
func resolve() {
	if v := os.Getenv("GREGAL_SHELL"); v != "" {
		resolved = v
		posix = filepath.Base(v) != "cmd" && filepath.Base(v) != "cmd.exe"
		return
	}
	if p, err := exec.LookPath("sh"); err == nil {
		resolved, posix = p, true
		return
	}
	if runtime.GOOS == "windows" {
		for _, c := range gitBashCandidates {
			if _, err := os.Stat(c); err == nil {
				resolved, posix = c, true
				return
			}
		}
		resolved, posix = "cmd", false
		return
	}
	resolved, posix = "sh", true
}

// Argv retorna l'executable i els arguments per executar cmd amb
// l'intèrpret de la màquina.
func Argv(cmd string) (string, []string) {
	once.Do(resolve)
	if posix {
		return resolved, []string{"-c", cmd}
	}
	return resolved, []string{"/C", cmd}
}

// reWinPath detecta rutes absolutes Windows (C:\..., C:/...) dins una
// ordre. El \b evita esquemes com https:// (allà la lletra va precedida
// de lletra) i el dígit inicial evita coses com "3:4".
var reWinPath = regexp.MustCompile(`\b([A-Za-z]):[\\/][^\s"'` + "`" + `]*`)

// NormalitzaPathsWindows tradueix C:\... → /c/... a l'ordre. Git Bash no
// entén la lletra d'unitat i el model escriu rutes Windows sempre: sense
// això cada ls/type amb ruta absoluta fallava. S'aplica a les ordres de
// l'agent (bash, processos), NO als hooks (allà {file} és literal).
// Fora de Windows+POSIX no es toca res.
func NormalitzaPathsWindows(cmd string) string {
	if runtime.GOOS != "windows" || !IsPOSIX() {
		return cmd
	}
	return reWinPath.ReplaceAllStringFunc(cmd, func(tok string) string {
		lletra := strings.ToLower(tok[:1])
		rest := tok[2:]
		// Puntuació final que és delimitador, no ruta ("...,", "....)").
		suf := ""
		for len(rest) > 0 && strings.ContainsRune(",;:!?)", rune(rest[len(rest)-1])) {
			suf = rest[len(rest)-1:] + suf
			rest = rest[:len(rest)-1]
		}
		rest = strings.ReplaceAll(rest, "\\", "/")
		return "/" + lletra + "/" + strings.Trim(rest, "/") + suf
	})
}

// Name descriu l'intèrpret triat, per als diagnòstics (doctor, errors).
func Name() string {
	once.Do(resolve)
	return resolved
}

// IsPOSIX diu si l'intèrpret entén sintaxi sh (&&, pipes, cometes simples).
func IsPOSIX() bool {
	once.Do(resolve)
	return posix
}
