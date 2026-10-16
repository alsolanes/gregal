// Package evals corre el banc de tasques d'evals/ des del mateix binari:
// `gregal --eval evals` (o --tasks t1,t3). Fins ara hi havia un run.sh
// (Linux) i un run.sh (Windows) amb la lògica duplicada i lligada
// a rutes de la màquina; cap dels dos donava una xifra que es pogués
// comparar entre versions. Aquí: una passada, una taula, un
// results/summary.json i, si hi ha evals/baseline.json, la llista de
// regressions. Sense això, cada canvi al prompt o al bucle és una aposta.
//
// Una tasca és tasks/<t>.prompt (la consigna), opcionalment tasks/<t>/
// o fixtures/<t>/ (fixture que es copia a results/<t>; la segona és la
// disposició de bench/), tasks/<t>.assert (script sh que rep el log JSON
// i l'expected; codi 0 = PASS) i tasks/<t>.expected. L'assert també rep
// GREGAL_EVAL_DIR (l'arrel del banc, per a proves amagades com
// bench/hidden) i GREGAL_EVAL_FIXTURE (la fixture original, per restaurar
// els tests abans de comprovar: un test retocat no pot fer passar res).
//
// Amb --repeat N cada tasca corre N cops: els models són estocàstics i un
// sol PASS/FAIL no distingeix una millora del soroll (stats.go).
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

// Result és el veredicte d'una execució d'una tasca amb les mètriques del
// torn.
type Result struct {
	Task    string   `json:"task"`
	Run     int      `json:"run,omitempty"` // 1..N amb --repeat
	Verdict string   `json:"verdict"`       // PASS | FAIL | ERROR | SENSE-ASSERT
	Detail  string   `json:"detail,omitempty"`
	Steps   int      `json:"steps"`
	Tools   []string `json:"tools,omitempty"`
	Up      int      `json:"up_tokens"`
	Down    int      `json:"down_tokens"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
	Routed  string   `json:"routed,omitempty"`
	Secs    int      `json:"secs"`
	// TokensSource és el de RunResult (api | mixed | estimate).
	// PromptTokens són els de pujada declarats pel proveïdor (0 si només
	// hi ha estimació): la base de cache_hit_rate.
	TokensSource      string `json:"tokens_source,omitempty"`
	PromptTokens      int    `json:"prompt_tokens"`
	CachedTokens      int    `json:"cached_tokens"`
	LLMCalls          int    `json:"llm_calls"`
	EditCalls         int    `json:"edit_calls"`
	EditFailed        int    `json:"edit_failed"`
	ToolCallInvalid   int    `json:"tool_call_invalid"`
	ToolCallRecovered int    `json:"tool_call_recovered"`
	DoomHits          int    `json:"doom_hits"`
	Compactions       int    `json:"compactions"`
	Exhausted         bool   `json:"exhausted,omitempty"`
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
		for _, fx := range []string{filepath.Join(tdir, name), filepath.Join(dir, "fixtures", name)} {
			if st, err := os.Stat(fx); err == nil && st.IsDir() {
				t.Fixture = fx
				break
			}
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
	// Repeat és quants cops corre cada tasca (<1 = 1).
	Repeat int
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
	if o.Repeat < 1 {
		o.Repeat = 1
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
		for run := 1; run <= o.Repeat; run++ {
			r, err := corre(ctx, cfg, client, dir, out, origDir, t, run, o, say)
			if err != nil {
				return results, err
			}
			// La fixture és sagrada: si el torn l'ha tocada (una ruta
			// absoluta, un cd ..), el veredicte no val i cal saber-ho.
			if t.Fixture != "" {
				if abans, ara := empremtes[t.Name], empremtaDir(t.Fixture); abans != "" && abans != ara {
					rel, _ := filepath.Rel(dir, t.Fixture)
					r.Verdict, r.Detail = "ERROR", "la tasca ha modificat la fixture "+filepath.ToSlash(rel)+" (restaura-la amb git)"
				}
			}
			results = append(results, r)
		}
	}
	if raw, err := json.MarshalIndent(Summarize(results, o.Repeat), "", "  "); err == nil {
		os.WriteFile(filepath.Join(out, "summary.json"), raw, 0o644)
	}
	return results, nil
}

// nomRun és el nom dels fitxers d'una execució: <t> amb una sola, <t>.rN
// amb --repeat (cada repetició té la seva còpia i el seu log).
func nomRun(t string, run, repeat int) string {
	if repeat <= 1 {
		return t
	}
	return fmt.Sprintf("%s.r%d", t, run)
}

// corre fa una execució d'una tasca en una còpia neta de la fixture. Un
// error vol dir que el banc no pot continuar (no es pot crear o entrar al
// directori de treball); el que falla de la tasca va al veredicte.
func corre(ctx context.Context, cfg *config.Config, client *llm.Client, dir, out, origDir string, t Task, run int, o Options, say func(string, ...any)) (Result, error) {
	r := Result{Task: t.Name}
	nom := nomRun(t.Name, run, o.Repeat)
	if o.Repeat > 1 {
		r.Run = run
	}
	work := filepath.Join(out, nom)
	os.RemoveAll(work)
	if t.Fixture != "" {
		if err := copiaDir(t.Fixture, work); err != nil {
			r.Verdict, r.Detail = "ERROR", "fixture: "+err.Error()
			return r, nil
		}
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return r, err
	}
	if o.Repeat > 1 {
		say("== %s (%d/%d)", t.Name, run, o.Repeat)
	} else {
		say("== %s", t.Name)
	}
	t0 := time.Now()
	tctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	// Les eines de l'agent treballen al directori del procés.
	if err := os.Chdir(work); err != nil {
		return r, err
	}
	res, err := agent.RunNonInteractiveEx(tctx, client, cfg, t.Prompt, "code", o.MaxSteps, true, func(step int, tools []string) {
		say("   pas %d: %s", step, strings.Join(tools, ", "))
	})
	os.Chdir(origDir)
	r.Secs = int(time.Since(t0).Seconds())
	ompleMetriques(&r, res)
	logPath := filepath.Join(out, nom+".log")
	if raw, merr := json.MarshalIndent(res, "", "  "); merr == nil {
		os.WriteFile(logPath, raw, 0o644)
	}
	if err != nil {
		r.Verdict, r.Detail = "ERROR", truncate(err.Error(), 200)
		say("   ERROR %s", r.Detail)
		return r, nil
	}
	switch {
	case t.Assert != "":
		ok, detall := assert(work, t.Assert, logPath, t.Expected, filepath.Join(out, nom+".assert.log"), dir, t.Fixture)
		if ok {
			r.Verdict = "PASS"
		} else {
			r.Verdict, r.Detail = "FAIL", detall
		}
	default:
		r.Verdict = "SENSE-ASSERT"
	}
	say("   %s (%d passos, %ds)", r.Verdict, r.Steps, r.Secs)
	return r, nil
}

// ompleMetriques copia del RunResult les mètriques que el banc agrega.
func ompleMetriques(r *Result, res agent.RunResult) {
	r.Steps, r.Tools, r.Up, r.Down, r.CostUSD = res.Steps, res.Tools, res.UpTokens, res.DownTokens, res.CostUSD
	if res.RoutedTo != "" {
		r.Routed = res.RoutedFrom + "->" + res.RoutedTo
	}
	r.TokensSource = res.TokensSource
	if res.TokensSource != "estimate" {
		r.PromptTokens = res.UpTokens
	}
	r.CachedTokens, r.LLMCalls = res.CachedTokens, res.LLMCalls
	r.EditCalls, r.EditFailed = res.EditCalls, res.EditFailed
	r.ToolCallInvalid, r.DoomHits, r.Compactions = res.ToolCallInvalid, res.DoomHits, res.Compactions
	r.ToolCallRecovered = res.ToolCallRecovered
	r.Exhausted = res.Exhausted
}

// assert executa l'script amb sh (Git Bash a Windows) dins del directori
// de la tasca, amb el log i l'expected com a arguments. A Windows, si no
// hi ha python3 però sí python, es posa un python3 de recanvi al PATH:
// els asserts el criden pel nom de sempre.
func assert(work, script, logPath, expected, outLog, evalDir, fixture string) (bool, string) {
	cmd := fmt.Sprintf("sh %q %q %q", posix(script), posix(logPath), posix(expected))
	prog, args := shell.Argv(cmd)
	c := exec.Command(prog, args...)
	c.Dir = work
	c.Env = append(ambPython3(os.Environ()), "GREGAL_EVAL_DIR="+posix(evalDir), "GREGAL_EVAL_FIXTURE="+posix(fixture))
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	err := c.Run()
	os.WriteFile(outLog, buf.Bytes(), 0o644)
	if err == nil {
		return true, ""
	}
	return false, truncate(strings.TrimSpace(darreraLinia(buf.String())), 200)
}

// posix és la ruta tal com la vol l'sh: a Windows el del Git, amb barres
// normals i lletra d'unitat traduïda (C:/x → /c/x), o no troba els fitxers.
func posix(p string) string {
	if runtime.GOOS != "windows" || p == "" {
		return p
	}
	return shell.NormalitzaPathsWindows(strings.ReplaceAll(p, "\\", "/"))
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
