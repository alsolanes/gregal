package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// El registre de torns (turns.jsonl, a la carpeta de dades) deixa per a cada
// torn quant ha trigat i on ha anat el temps, per poder comparar dies («ahir
// anava més ràpid?») amb números en lloc d'impressions. Només hi ha temps,
// comptadors i el model: mai el text de la conversa ni arguments d'eines.

type turnRecord struct {
	Ts           string `json:"ts"`
	Kind         string `json:"kind"` // agent, wind, wind-task (encàrrec de l'agent), rotllana (una línia per veu)
	ID           string `json:"id,omitempty"`
	Model        string `json:"model,omitempty"`
	FirstTokenMs int64  `json:"first_token_ms,omitempty"`
	TotalMs      int64  `json:"total_ms"`
	Steps        int    `json:"steps,omitempty"`
	ModelMs      int64  `json:"model_ms,omitempty"`
	ToolsMs      int64  `json:"tools_ms,omitempty"`
	Prompt       int    `json:"prompt_tokens,omitempty"`
	Completion   int    `json:"completion_tokens,omitempty"`
	Cached       int    `json:"cached_prompt_tokens,omitempty"`
	ModelCalls   int    `json:"model_calls,omitempty"`
	UsageCalls   int    `json:"usage_calls,omitempty"`
	UsageSource  string `json:"usage_source,omitempty"`
	Outcome      string `json:"outcome"` // ok, partial, stopped, error
	Error        string `json:"error,omitempty"`
}

// turnLogMaxBytes: en passar-ho, el fitxer es mou a turns.jsonl.1 (només es
// guarda l'anterior). Uns 20.000 torns.
const turnLogMaxBytes = 5 << 20

var turnLogMu sync.Mutex

func turnLogPath() string { return filepath.Join(filepath.Dir(session.DefaultDir()), "turns.jsonl") }

type turnLog struct {
	mu         sync.Mutex
	rec        turnRecord
	start      time.Time
	first      time.Time
	steps      int
	prompt     int
	completion int
	meter      *llm.UsageMeter
}

func startTurnLog(kind, id string, role config.Role) *turnLog {
	t := &turnLog{rec: turnRecord{Kind: kind, ID: id}, start: time.Now()}
	t.setRole(role)
	return t
}

func (t *turnLog) setRole(role config.Role) {
	model := role.Model
	if role.Provider != "" && model != "" {
		model = role.Provider + "/" + model
	}
	t.rec.Model = model
}

func (t *turnLog) withUsage(ctx context.Context) context.Context {
	t.meter = &llm.UsageMeter{}
	return llm.WithUsage(ctx, t.meter)
}

// token apunta el primer tros de resposta del torn.
func (t *turnLog) token() {
	t.mu.Lock()
	if t.first.IsZero() {
		t.first = time.Now()
	}
	t.mu.Unlock()
}

// emitter embolica l'emissor d'un torn d'agent: el primer «token» marca el
// temps fins a la primera paraula i cada «speed» és una crida al model.
func (t *turnLog) emitter(emit func(string, any)) func(string, any) {
	return func(kind string, payload any) {
		switch kind {
		case "token":
			t.token()
		case "speed":
			if ev, ok := payload.(map[string]any); ok {
				t.mu.Lock()
				t.steps++
				t.prompt += asInt(ev["prompt"])
				t.completion += asInt(ev["completion"])
				t.mu.Unlock()
			}
		}
		emit(kind, payload)
	}
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// finish tanca el torn i l'escriu. tt (pot ser nil) porta els passos i el
// temps de model i d'eines dels loops dels vents.
func (t *turnLog) finish(err error, stopped, partial bool, tt *tempsTorn) {
	if t == nil {
		return
	}
	end := time.Now()
	t.mu.Lock()
	rec := t.rec
	rec.Ts = t.start.UTC().Format(time.RFC3339)
	rec.TotalMs = end.Sub(t.start).Milliseconds()
	if !t.first.IsZero() {
		rec.FirstTokenMs = t.first.Sub(t.start).Milliseconds()
	}
	rec.Steps, rec.Prompt, rec.Completion = t.steps, t.prompt, t.completion
	t.mu.Unlock()
	if tt != nil {
		rec.Steps = tt.passos
		rec.ModelMs, rec.ToolsMs = tt.model.Milliseconds(), tt.eines.Milliseconds()
	}
	if t.meter != nil {
		u, calls, measured := t.meter.Total()
		rec.ModelCalls, rec.UsageCalls = calls, measured
		switch {
		case measured > 0:
			rec.Prompt, rec.Cached, rec.Completion = u.PromptTokens, u.PromptTokensDetails.CachedTokens, u.CompletionTokens
			rec.UsageSource = "api"
			if measured < calls {
				rec.UsageSource = "mixed"
			}
		case rec.Prompt > 0 || rec.Completion > 0:
			rec.UsageSource = "estimate"
		case calls > 0:
			rec.UsageSource = "unreported"
		}
	}
	switch {
	case stopped:
		rec.Outcome = "stopped"
	case err != nil:
		rec.Outcome = "error"
		rec.Error = workshopText(redactError(err), 200)
	case partial:
		rec.Outcome = "partial"
	default:
		rec.Outcome = "ok"
	}
	appendTurnLog(rec)
}

func appendTurnLog(rec turnRecord) {
	// Les proves sense GREGAL_DATA_DIR no han d'escriure a les dades de debò.
	if testing.Testing() && strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")) == "" {
		return
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	turnLogMu.Lock()
	defer turnLogMu.Unlock()
	path := turnLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size() > turnLogMaxBytes {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}
