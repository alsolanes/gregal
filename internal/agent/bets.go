package agent

// Apostes de confiança: abans de córrer una comprovació (tests, build,
// vet, lint) l'agent diu amb quina probabilitat creu que passarà. El
// resultat real es desa al costat de la predicció i, amb el temps, en surt
// una calibració (Brier): un agent que diu 90% i falla la meitat de cops
// està massa segur, i això val més que qualsevol autoavaluació en text.
//
// La predicció viatja dins de la mateixa crida (paràmetre opcional «bet» de
// bash): no costa cap crida de més. Amb agent.bets: side, quan el model no
// l'ha donada, el conductor en pot demanar una de lateral al rol chat
// (ApostaLateral). agent.bets: off ho apaga tot.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// betParam és el paràmetre opcional «bet» de l'eina bash.
var betParam = map[string]any{
	"type":        "integer",
	"description": "Only when the command is a verification (tests, build, vet, lint): your calibrated probability, 0-100, that it passes. Be honest; it is scored against the real outcome.",
}

// Aposta és una predicció resolta.
type Aposta struct {
	TS      string  `json:"ts"`
	Session string  `json:"session,omitempty"`
	Project string  `json:"project,omitempty"`
	Command string  `json:"command"`
	P       float64 `json:"p"`
	Passed  bool    `json:"passed"`
	// Source: "model" (paràmetre bet) o "side" (crida lateral).
	Source string `json:"source"`
}

// ApostaFunc demana una predicció lateral per a una comprovació que el
// model ha llançat sense «bet». ok=false = sense predicció.
type ApostaFunc func(ctx context.Context, command string) (p float64, ok bool)

// BetsMode torna el mode efectiu de les apostes: on | off | side.
func BetsMode(cfg *config.Config) string {
	if cfg == nil {
		return "on"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Agent.Bets)) {
	case "off", "false", "no":
		return "off"
	case "side":
		return "side"
	}
	return "on"
}

// ParseBet normalitza una predicció: 0-1 o 0-100, nombre o text («80%»).
func ParseBet(raw any) (float64, bool) {
	var v float64
	switch x := raw.(type) {
	case float64:
		v = x
	case int:
		v = float64(x)
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		v = f
	case string:
		t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(x), "%"))
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		v = f
	default:
		return 0, false
	}
	if math.IsNaN(v) || v < 0 {
		return 0, false
	}
	if v > 1 {
		v /= 100
	}
	if v > 1 {
		return 0, false
	}
	return v, true
}

// apostaDeArgs llegeix «bet» i «command» dels arguments d'una crida bash.
func apostaDeArgs(argsJSON string) (cmd string, p float64, ok bool) {
	var a map[string]any
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return "", 0, false
	}
	cmd, _ = a["command"].(string)
	if raw, hi := a["bet"]; hi {
		p, ok = ParseBet(raw)
	}
	return cmd, p, ok
}

// BetChip és el text curt de la fila de l'eina: «bet 80% ✓».
func BetChip(a Aposta) string {
	mark := "✗"
	if a.Passed {
		mark = "✓"
	}
	return fmt.Sprintf("bet %d%% %s", int(math.Round(a.P*100)), mark)
}

// --- registre ---

var (
	apostesMu sync.Mutex
	// BetsPathOverride redirigeix el registre (tests).
	BetsPathOverride string
)

// BetsPath és el JSONL de les apostes: <data>/bets.jsonl (GREGAL_DATA_DIR
// o ~/.local/share/gregal, com les converses).
func BetsPath() string {
	if BetsPathOverride != "" {
		return BetsPathOverride
	}
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return filepath.Join(d, "bets.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "gregal", "bets.jsonl")
}

// RegistraAposta afegeix una aposta al registre (best-effort).
func RegistraAposta(a Aposta) error {
	path := BetsPath()
	if path == "" {
		return nil
	}
	if a.TS == "" {
		a.TS = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	apostesMu.Lock()
	defer apostesMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

// LlegeixApostes torna les apostes del registre que passen el filtre
// (sessió i/o projecte buits = totes).
func LlegeixApostes(session, project string) []Aposta {
	path := BetsPath()
	if path == "" {
		return nil
	}
	apostesMu.Lock()
	defer apostesMu.Unlock()
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Aposta
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var a Aposta
		if json.Unmarshal(sc.Bytes(), &a) != nil {
			continue
		}
		if session != "" && a.Session != session {
			continue
		}
		if project != "" && !mateixProjecte(a.Project, project) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func mateixProjecte(a, b string) bool {
	na := strings.ToLower(filepath.Clean(a))
	nb := strings.ToLower(filepath.Clean(b))
	return na == nb
}

// Calibracio és el resum d'un conjunt d'apostes.
type Calibracio struct {
	Count int `json:"count"`
	// Brier: mitjana de (p - resultat)². 0 = perfecte; 0.25 = dir sempre 50%.
	Brier    *float64 `json:"brier"`
	MeanP    *float64 `json:"mean_p"`
	PassRate *float64 `json:"pass_rate"`
	// Overconfidence = MeanP - PassRate (positiu: massa optimista).
	Overconfidence *float64      `json:"overconfidence"`
	Buckets        []CalibBucket `json:"buckets"`
	Recent         []Aposta      `json:"recent"`
}

// CalibBucket agrupa les apostes per franges de probabilitat de 20 punts.
type CalibBucket struct {
	From     int     `json:"from"`
	To       int     `json:"to"`
	Count    int     `json:"count"`
	MeanP    float64 `json:"mean_p"`
	PassRate float64 `json:"pass_rate"`
}

// Calibra calcula el resum (Brier, franges i les últimes apostes).
func Calibra(list []Aposta) Calibracio {
	c := Calibracio{Count: len(list), Buckets: []CalibBucket{}, Recent: []Aposta{}}
	if len(list) == 0 {
		return c
	}
	var brier, sumP, pass float64
	type acc struct {
		n         int
		sumP, pas float64
	}
	buckets := make([]acc, 5)
	for _, a := range list {
		o := 0.0
		if a.Passed {
			o = 1
			pass++
		}
		brier += (a.P - o) * (a.P - o)
		sumP += a.P
		i := int(a.P * 5)
		if i > 4 {
			i = 4
		}
		buckets[i].n++
		buckets[i].sumP += a.P
		buckets[i].pas += o
	}
	n := float64(len(list))
	b, m, r := round3(brier/n), round3(sumP/n), round3(pass/n)
	over := round3(m - r)
	c.Brier, c.MeanP, c.PassRate, c.Overconfidence = &b, &m, &r, &over
	for i, x := range buckets {
		if x.n == 0 {
			continue
		}
		c.Buckets = append(c.Buckets, CalibBucket{From: i * 20, To: i*20 + 20, Count: x.n,
			MeanP: round3(x.sumP / float64(x.n)), PassRate: round3(x.pas / float64(x.n))})
	}
	start := len(list) - 10
	if start < 0 {
		start = 0
	}
	c.Recent = append(c.Recent, list[start:]...)
	return c
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// --- aposta lateral ---

var reProb = regexp.MustCompile(`(\d{1,3}(?:\.\d+)?)\s*%?`)

// ApostaPrompt és la consigna de la crida lateral.
func ApostaPrompt(command, diff string) string {
	if len(diff) > 6000 {
		diff = diff[:6000] + "\n…(truncated)"
	}
	return "You predict whether a verification command will pass in this repository given the current uncommitted changes.\n" +
		"Reply with ONLY an integer from 0 to 100: the calibrated probability that the command exits successfully.\n\n" +
		"COMMAND:\n" + command + "\n\nDIFF:\n" + diff
}

// ParseApostaResposta extreu el número de la resposta lateral.
func ParseApostaResposta(text string) (float64, bool) {
	m := reProb.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	return ParseBet(m[1])
}

// ApostaLateral és el ganxo de agent.bets: side: demana la predicció al rol
// chat amb el diff i l'ordre. Torna nil si el mode no és side o no hi ha rol.
func ApostaLateral(client *llm.Client, cfg *config.Config, dir string) ApostaFunc {
	if client == nil || cfg == nil || BetsMode(cfg) != "side" {
		return nil
	}
	r, ok := cfg.Roles["chat"]
	if !ok {
		return nil
	}
	p, ok := cfg.Providers[r.Provider]
	if !ok {
		return nil
	}
	return func(ctx context.Context, command string) (float64, bool) {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		out, _, err := client.ChatFO(cctx, cfg.PrimTarget(p, r), cfg.FallbackTarget(r),
			[]llm.Message{{Role: "user", Content: ApostaPrompt(command, AutonomousDiff(dir))}}, 0, 16, nil)
		if err != nil {
			return 0, false
		}
		return ParseApostaResposta(out)
	}
}
