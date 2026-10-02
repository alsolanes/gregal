// Package evals corre el banc de tasques d'evals/ des del mateix binari:
// `gregal --eval evals` (o --tasks t1,t3). Fins ara hi havia un run.sh
// (Linux) i un run.sh (Windows) amb la lògica duplicada i lligada
// a rutes de la màquina; cap dels dos donava una xifra que es pogués
// comparar entre versions. Aquí: una passada, una taula, un
// results/summary.json i, si hi ha evals/baseline.json, la llista de
// regressions. Sense això, cada canvi al prompt o al bucle és una aposta.
//
// Una tasca és tasks/<t>.prompt (la consigna), opcionalment tasks/<t>/
// (fixture que es copia a results/<t>), tasks/<t>.assert (script sh que
// rep el log JSON i l'expected; codi 0 = PASS) i tasks/<t>.expected.
package evals

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/shell"
)

// Task és una tasca del banc.
type Task struct {
	Name     string
	Prompt   string
	Fixture  string // directori a copiar ("" si no n'hi ha)
	Assert   string // script ("" si no n'hi ha)
	Expected string // fitxer esperat ("" si no n'hi ha)
}

// Result és el veredicte d'una tasca amb les mètriques del torn.
type Result struct {
	Task    string   `json:"task"`
	Verdict string   `json:"verdict"` // PASS | FAIL | ERROR | SENSE-ASSERT
	Detail  string   `json:"detail,omitempty"`
	Steps   int      `json:"steps"`
	Tools   []string `json:"tools,omitempty"`
	Up      int      `json:"up_tokens"`
	Down    int      `json:"down_tokens"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
	Routed  string   `json:"routed,omitempty"`
	Secs    int      `json:"secs"`
}

// Load llegeix les tasques de <dir>/tasks (ordenades pel nom).
func Load(dir string) ([]Task, error) {
	// Rutes absolutes: els asserts corren amb el directori de la tasca
	// com a cwd, i una ruta relativa al repo allà no hi és.
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	tdir := filepath.Join(dir, "tasks")
	entries, err := os.ReadDir(tdir)
	if err != nil {
		return nil, fmt.Errorf("evals: no trobo %s: %w", tdir, err)
	}
	var out []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".prompt") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".prompt")
		raw, err := os.ReadFile(filepath.Join(tdir, e.Name()))
		if err != nil {
			return nil, err
		}
		t := Task{Name: name, Prompt: strings.TrimSpace(string(raw))}
		if st, err := os.Stat(filepath.Join(tdir, name)); err == nil && st.IsDir() {
			t.Fixture = filepath.Join(tdir, name)
		}
		if _, err := os.Stat(filepath.Join(tdir, name+".assert")); err == nil {
			t.Assert = filepath.Join(tdir, name+".assert")
		}
		if _, err := os.Stat(filepath.Join(tdir, name+".expected")); err == nil {
			t.Expected = filepath.Join(tdir, name+".expected")
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, fmt.Errorf("evals: cap tasca (*.prompt) a %s", tdir)
	}
	return out, nil
}

// Options configura una passada.
type Options struct {
	MaxSteps int
	// Names limita les tasques (buit = totes).
	Names []string
	// OnEvent rep línies de progrés (stderr al CLI).
	OnEvent func(string)
	// Timeout per tasca (agent + assert).
	Timeout time.Duration
}

// Run corre les tasques en còpies sota <dir>/results i torna els
// resultats en ordre. No toca mai tasks/.
func Run(ctx context.Context, cfg *config.Config, client *llm.Client, dir string, tasks []Task, o Options) ([]Result, error) {
	if o.MaxSteps <= 0 {
		o.MaxSteps = 25
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Minute
	}
	say := func(f string, a ...any) {
		if o.OnEvent != nil {
			o.OnEvent(fmt.Sprintf(f, a...))
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	out := filepath.Join(dir, "results")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	vol := map[string]bool{}
	for _, n := range o.Names {
		if n = strings.TrimSpace(n); n != "" {
			vol[n] = true
		}
	}
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	empremtes := map[string]string{}
	for _, t := range tasks {
		if t.Fixture != "" {
			empremtes[t.Name] = empremtaDir(t.Fixture)
		}
	}
	var results []Result
	for _, t := range tasks {
		if len(vol) > 0 && !vol[t.Name] {
			continue
		}
		r := Result{Task: t.Name}
		work := filepath.Join(out, t.Name)
		os.RemoveAll(work)
		if t.Fixture != "" {
			if err := copiaDir(t.Fixture, work); err != nil {
				r.Verdict, r.Detail = "ERROR", "fixture: "+err.Error()
				results = append(results, r)
				continue
			}
		} else if err := os.MkdirAll(work, 0o755); err != nil {
			return results, err
		}
		say("== %s", t.Name)
		t0 := time.Now()
		tctx, cancel := context.WithTimeout(ctx, o.Timeout)
		// Les eines de l'agent treballen al directori del procés.
		if err := os.Chdir(work); err != nil {
			cancel()
			return results, err
		}
		res, err := agent.RunNonInteractiveEx(tctx, client, cfg, t.Prompt, "code", o.MaxSteps, true, func(step int, tools []string) {
			say("   pas %d: %s", step, strings.Join(tools, ", "))
		})
		cancel()
		os.Chdir(origDir)
		r.Secs = int(time.Since(t0).Seconds())
		r.Steps, r.Tools, r.Up, r.Down, r.CostUSD = res.Steps, res.Tools, res.UpTokens, res.DownTokens, res.CostUSD
		if res.RoutedTo != "" {
			r.Routed = res.RoutedFrom + "->" + res.RoutedTo
		}
		logPath := filepath.Join(out, t.Name+".log")
		if raw, merr := json.MarshalIndent(res, "", "  "); merr == nil {
			os.WriteFile(logPath, raw, 0o644)
		}
		if err != nil {
			r.Verdict, r.Detail = "ERROR", truncate(err.Error(), 200)
			results = append(results, r)
			say("   ERROR %s", r.Detail)
			continue
		}
		switch {
		case t.Assert != "":
			ok, detall := assert(work, t.Assert, logPath, t.Expected, filepath.Join(out, t.Name+".assert.log"))
			if ok {
				r.Verdict = "PASS"
			} else {
				r.Verdict, r.Detail = "FAIL", detall
			}
		default:
			r.Verdict = "SENSE-ASSERT"
		}
		// La fixture és sagrada: si el torn l'ha tocada (una ruta absoluta,
		// un cd ..), el veredicte no val i cal saber-ho.
		if t.Fixture != "" {
			if abans, ara := empremtes[t.Name], empremtaDir(t.Fixture); abans != "" && abans != ara {
				r.Verdict, r.Detail = "ERROR", "la tasca ha modificat la fixture tasks/"+t.Name+" (restaura-la amb git)"
			}
		}
		say("   %s (%d passos, %ds)", r.Verdict, r.Steps, r.Secs)
		results = append(results, r)
	}
	if raw, err := json.MarshalIndent(results, "", "  "); err == nil {
		os.WriteFile(filepath.Join(out, "summary.json"), raw, 0o644)
	}
	return results, nil
}

// assert executa l'script amb sh (Git Bash a Windows) dins del directori
// de la tasca, amb el log i l'expected com a arguments. A Windows, si no
// hi ha python3 però sí python, es posa un python3 de recanvi al PATH:
// els asserts el criden pel nom de sempre.
func assert(work, script, logPath, expected, outLog string) (bool, string) {
	// A Windows l'sh és el del Git: barres normals i lletra d'unitat
	// traduïda (C:/x → /c/x), o no troba els fitxers.
	posix := func(p string) string {
		if runtime.GOOS != "windows" || p == "" {
			return p
		}
		return shell.NormalitzaPathsWindows(strings.ReplaceAll(p, "\\", "/"))
	}
	cmd := fmt.Sprintf("sh %q %q %q", posix(script), posix(logPath), posix(expected))
	prog, args := shell.Argv(cmd)
	c := exec.Command(prog, args...)
	c.Dir = work
	c.Env = ambPython3(os.Environ())
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	err := c.Run()
	os.WriteFile(outLog, buf.Bytes(), 0o644)
	if err == nil {
		return true, ""
	}
	return false, truncate(strings.TrimSpace(darreraLinia(buf.String())), 200)
}

// ambPython3 afegeix un directori amb un `python3` que crida `python`
// quan només hi ha el segon (Windows).
func ambPython3(env []string) []string {
	if _, err := exec.LookPath("python3"); err == nil {
		return env
	}
	py, err := exec.LookPath("python")
	if err != nil {
		return env
	}
	dir := filepath.Join(os.TempDir(), "gregal-py3-shim")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return env
	}
	shim := filepath.Join(dir, "python3")
	if _, err := os.Stat(shim); err != nil {
		pyPosix := py
		if runtime.GOOS == "windows" {
			pyPosix = shell.NormalitzaPathsWindows(strings.ReplaceAll(py, "\\", "/"))
		}
		os.WriteFile(shim, []byte("#!/bin/sh\nexec \""+pyPosix+"\" \"$@\"\n"), 0o755)
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			out = append(out, e[:5]+dir+string(os.PathListSeparator)+e[5:])
			continue
		}
		out = append(out, e)
	}
	return out
}

// empremtaDir resumeix el contingut d'un directori (noms + bytes) per
// detectar que una tasca l'ha tocat.
func empremtaDir(dir string) string {
	h := sha256.New()
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		if strings.Contains(rel, "__pycache__") {
			return nil
		}
		io.WriteString(h, rel+"|")
		raw, _ := os.ReadFile(p)
		h.Write(raw)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

func darreraLinia(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return s
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func copiaDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		outF, err := os.Create(target)
		if err != nil {
			return err
		}
		defer outF.Close()
		_, err = io.Copy(outF, in)
		return err
	})
}

// Baseline és tasca → veredicte de la passada de referència
// (evals/baseline.json).
type Baseline map[string]string

// LoadBaseline llegeix <dir>/baseline.json (nil si no hi és).
func LoadBaseline(dir string) Baseline {
	raw, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	if err != nil {
		return nil
	}
	var b Baseline
	if json.Unmarshal(raw, &b) != nil {
		return nil
	}
	return b
}

// SaveBaseline desa els veredictes com a referència nova.
func SaveBaseline(dir string, results []Result) error {
	b := Baseline{}
	for _, r := range results {
		b[r.Task] = r.Verdict
	}
	raw, _ := json.MarshalIndent(b, "", "  ")
	return os.WriteFile(filepath.Join(dir, "baseline.json"), raw, 0o644)
}

// Report pinta la taula, el recompte i les diferències amb la referència.
func Report(w io.Writer, results []Result, base Baseline) (passed, total int, regressions []string) {
	fmt.Fprintf(w, "%-6s %-12s %6s %8s %5s  %-28s %s\n", "tasca", "veredicte", "passos", "tokens", "s", "eines", "detall")
	for _, r := range results {
		eines := strings.Join(r.Tools, ",")
		if len(eines) > 28 {
			eines = eines[:27] + "…"
		}
		fmt.Fprintf(w, "%-6s %-12s %6d %8d %5d  %-28s %s\n", r.Task, r.Verdict, r.Steps, r.Up+r.Down, r.Secs, eines, r.Detail)
		total++
		if r.Verdict == "PASS" {
			passed++
		}
		if base != nil {
			if was, ok := base[r.Task]; ok && was == "PASS" && r.Verdict != "PASS" {
				regressions = append(regressions, r.Task)
			}
		}
	}
	fmt.Fprintf(w, "\n%d/%d PASS", passed, total)
	if base != nil {
		if len(regressions) > 0 {
			fmt.Fprintf(w, " · REGRESSIONS respecte baseline: %s", strings.Join(regressions, ", "))
		} else {
			fmt.Fprintf(w, " · sense regressions respecte baseline")
		}
	} else {
		fmt.Fprintf(w, " · sense baseline (desa-la amb --eval-baseline)")
	}
	fmt.Fprintln(w)
	return passed, total, regressions
}
