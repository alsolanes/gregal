package agent

// Mode debug: depurar amb evidència en temps d'execució en comptes
// d'endevinar. El model fa hipòtesis, hi posa instrumentació temporal
// marcada amb DebugTag, reprodueix (o demana reproduir), llegeix el que ha
// sortit, arregla la causa i treu tota la instrumentació. L'últim pas no
// es deixa a la bona fe del model: abans de tancar, el motor busca la marca
// al workspace (debugGate a motor_caps.go) i, si n'hi ha de nova, el torna
// a enviar a netejar.

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DebugTag marca cada línia d'instrumentació temporal del mode debug.
const DebugTag = "GREGAL-DEBUG"

// debugRules són les regles del mode debug: les de code i el flux de
// depuració. Escriure segueix la política normal del mode code.
const debugRules = codeRules + `

DEBUG MODE (find the root cause with runtime evidence)
1. Hypotheses first: from the bug report and the code, write 2-4 ranked hypotheses (most likely first) before changing anything.
2. Instrument: add minimal, targeted temporary logging or prints that would confirm or refute each hypothesis. EVERY instrumentation line must contain the marker ` + DebugTag + ` (in the logged text or in a trailing comment) so it can be found and removed.
3. Reproduce: if the user gave a reproduction command or one is obvious (a failing test, a script, a CLI call), run it yourself. Otherwise ask the user to reproduce the bug and paste the output, and stop there (say that the instrumentation stays in place until then).
4. Read the evidence: look at the output and logs and decide which hypotheses are confirmed or refuted. Never fix on a guess the evidence contradicts; if all hypotheses are refuted, form new ones and instrument again.
5. Fix the root cause with the smallest change.
6. Clean up: remove ALL ` + DebugTag + ` instrumentation, confirm with a search (grep for ` + DebugTag + `) that none remains, and run the relevant checks.
7. Final answer: list every hypothesis as CONFIRMED or REFUTED with the evidence that decided it (the log line or output), then the fix (file and change) and the checks you ran with their result.`

// GuiaDebugNeteja és la guia quan el model tanca amb instrumentació nova.
func GuiaDebugNeteja(llocs []string) string {
	return "DEBUG MODE CHECK: " + DebugTag + " instrumentation is still present in: " + strings.Join(llocs, ", ") +
		". If the bug is fixed, remove every " + DebugTag + " line, grep for " + DebugTag + " to confirm none remains, run the checks and then give the final answer. " +
		"If you are still waiting for the user to reproduce the bug, say so explicitly in your answer and leave the instrumentation in place."
}

// DebugTagCounts compta les línies amb DebugTag per fitxer (camí relatiu
// amb barres /). Fa servir git grep (fitxers seguits i nous no ignorats);
// fora d'un repo, recorre el directori amb topalls.
func DebugTagCounts(dir string) map[string]int {
	if strings.TrimSpace(dir) == "" {
		return map[string]int{}
	}
	if m, ok := debugTagsGit(dir); ok {
		return m
	}
	return debugTagsWalk(dir)
}

func debugTagsGit(dir string) (map[string]int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "grep", "-c", "-I", "--untracked", "-F", "--full-name", DebugTag, "--", ".")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		// git grep torna 1 quan no hi ha cap coincidència.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return map[string]int{}, true
		}
		return nil, false
	}
	m := map[string]int{}
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		line := sc.Text()
		i := strings.LastIndexByte(line, ':')
		if i <= 0 {
			continue
		}
		n, err := strconv.Atoi(line[i+1:])
		if err != nil {
			continue
		}
		m[filepath.ToSlash(line[:i])] = n
	}
	return m, true
}

func debugTagsWalk(dir string) map[string]int {
	m := map[string]int{}
	vistos := 0
	tag := []byte(DebugTag)
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", ".gregal", "dist", "build", "target", "__pycache__":
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		vistos++
		if vistos > 5000 {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(raw, 0) >= 0 {
			return nil
		}
		n := 0
		for _, l := range bytes.Split(raw, []byte("\n")) {
			if bytes.Contains(l, tag) {
				n++
			}
		}
		if n > 0 {
			rel, _ := filepath.Rel(dir, path)
			m[filepath.ToSlash(rel)] = n
		}
		return nil
	})
	return m
}

// DebugTagsNous torna els fitxers on hi ha més marques que a l'inici del
// torn (un projecte que ja conté la cadena, com el mateix gregal, no ha de
// disparar el gate), ordenats.
func DebugTagsNous(base, ara map[string]int) []string {
	var out []string
	for f, n := range ara {
		if n > base[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
