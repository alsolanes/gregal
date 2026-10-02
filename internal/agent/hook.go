package agent

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gregal/internal/shell"
)

// postEditCmd és el hook post-escriptura (estil PostToolUse de Claude Code):
// comanda plantilla amb {file} que corre després de cada write/edit amb
// èxit. "" = desactivat. Es configura des del config (hooks.post_edit) a
// cada punt d'arrencada (TUI, serve, telegram, headless).
var postEditCmd atomic.Value // string

// SetPostEditHook desa el hook ("" el desactiva). Segur per concurrència.
func SetPostEditHook(cmd string) { postEditCmd.Store(strings.TrimSpace(cmd)) }

func getPostEditHook() string {
	v, _ := postEditCmd.Load().(string)
	return v
}

// runPostEditHook executa el hook amb el fitxer tocat. Torna "" si no hi ha
// hook o no diu res. Un hook que falla no fa fallar l'eina: s'anota i prou
// (l'agent decideix si cal reaccionar).
func runPostEditHook(path string) string {
	cmd := getPostEditHook()
	if cmd == "" {
		return ""
	}
	var fileArg string
	if shell.IsPOSIX() {
		fileArg = shQuote(path)
	} else {
		fileArg = `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	shellCmd := strings.ReplaceAll(cmd, "{file}", fileArg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prog, args := shell.Argv(shellCmd)
	c := exec.CommandContext(ctx, prog, args...)
	c.Dir = filepath.Dir(path)
	raw, err := c.CombinedOutput()
	out := strings.TrimSpace(truncateHook(string(raw)))
	if ctx.Err() == context.DeadlineExceeded {
		return "[hook] temps exhaurit (30s)"
	}
	if err != nil {
		if out == "" {
			out = strings.TrimSpace(err.Error())
		}
		return "[hook] avís: " + out
	}
	if out == "" {
		return ""
	}
	return "[hook] " + out
}

// shQuote embolcalla en cometes simples (segur per a sh -c).
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func truncateHook(s string) string {
	const max = 1500
	if len(s) <= max {
		return s
	}
	return s[:max] + "…(retallat)"
}

// apendixEdicio és el que s'afegeix al resultat de write/edit/patch: la
// comprovació de sintaxi primer i el hook de l'usuari després. Tots tres
// camins hi passen, que abans cadascun repetia les mateixes quatre línies
// i era qüestió de temps que un se n'oblidés.
//
// La sintaxi va davant del hook a posta: si el fitxer ha quedat trencat,
// això és el que el model ha de llegir primer.
func apendixEdicio(path string) string {
	var parts []string
	if d := Diagnostica(path); d != "" {
		parts = append(parts, d)
	}
	if h := runPostEditHook(path); h != "" {
		parts = append(parts, h)
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n" + strings.Join(parts, "\n")
}
