package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Errors de tipus de Python (el que un LSP com Pylance/pyright diria al
// moment). El ruff/pyflakes de diagPythonLint només veu noms no definits;
// pyright (o mypy) veu «argument de tipus str on cal int», atributs que no
// existeixen, imports trencats. Com la resta de diag_exec, amb l'eina de
// línia d'ordres si és a la màquina i en silenci si no hi és.

func diagPythonTypes(path string) string {
	dir, base := filepath.Dir(path), filepath.Base(path)
	if exe, ok := trobaBinari("pyright"); ok {
		r := execCurt(DiagExecTimeout, dir, nil, exe, "--outputjson", base)
		if !r.acabat {
			return ""
		}
		return parsePyright(r.out, base)
	}
	if exe, ok := trobaBinari("mypy"); ok {
		r := execCurt(DiagExecTimeout, dir, nil, exe, "--no-error-summary", "--hide-error-context", "--follow-imports=silent", "--ignore-missing-imports", base)
		if !r.acabat || r.err == nil {
			return ""
		}
		var linies []string
		for _, l := range strings.Split(r.out, "\n") {
			if strings.HasPrefix(l, base+":") && strings.Contains(l, ": error:") {
				linies = append(linies, strings.TrimSpace(l))
			}
			if len(linies) >= 8 {
				break
			}
		}
		if len(linies) == 0 {
			return ""
		}
		return "[tipus] mypy:\n" + strings.Join(linies, "\n")
	}
	return ""
}

// nomFitxer és el nom del fitxer d'un camí amb / o \ (pyright escriu el
// camí del sistema on corre; filepath.Base només coneix el separador local).
func nomFitxer(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// parsePyright extreu els errors del fitxer de la sortida JSON de pyright.
func parsePyright(raw, base string) string {
	var out struct {
		Diags []struct {
			File     string `json:"file"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
			Range    struct {
				Start struct{ Line int } `json:"start"`
			} `json:"range"`
		} `json:"generalDiagnostics"`
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return ""
	}
	var linies []string
	for _, d := range out.Diags {
		if d.Severity != "error" || !strings.EqualFold(nomFitxer(d.File), base) {
			continue
		}
		msg := strings.SplitN(d.Message, "\n", 2)[0]
		linies = append(linies, fmt.Sprintf("%s:%d: %s", base, d.Range.Start.Line+1, msg))
		if len(linies) >= 8 {
			break
		}
	}
	if len(linies) == 0 {
		return ""
	}
	return "[tipus] pyright:\n" + strings.Join(linies, "\n")
}
