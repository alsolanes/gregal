package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// delegateToolSpec és l'eina de subagents paral·lels (estil Task de Claude
// Code). v1: exploració read-only (mode inspect); els subagents no poden
// escriure ni delegar (sense recursió).
func delegateToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{
		Name:        "delegate",
		Description: "Llança 1-4 subagents en paral·lel per explorar (lectura: read, grep, glob, web_*). Cadascun torna un resum. Útil per investigar diverses pistes alhora.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"tasks": map[string]any{"type": "array", "items": map[string]any{"type": "object",
				"properties": map[string]any{"prompt": str}, "required": []string{"prompt"}}},
		}, "required": []string{"tasks"}},
	}
}

// delegateRunner fa la feina real d'un subagent. Es configura a l'arrencada
// (mateix provider/model del rol code); nil = no disponible.
var delegateRunner func(ctx context.Context, prompt string) (string, error)

// SetDelegateRunner configura (o neteja amb nil) el runner de subagents.
func SetDelegateRunner(fn func(ctx context.Context, prompt string) (string, error)) {
	delegateRunner = fn
}

// MaxDelegateTasks topa els subagents per crida (cada un fa els seus passos:
// 4 ja són prou context de tornada).
const MaxDelegateTasks = 4

// SetupDelegate configura el runner amb el rol code del config (mateix
// provider/model que l'agent pare). Els subagents exploren en mode inspect
// (12 passos, eines natives sense delegate ni MCP, ask→deny): no escriuen,
// no demanen res, només tornen text.
func SetupDelegate(cfg *config.Config, client *llm.Client) {
	r, ok := cfg.Roles["code"]
	if !ok {
		return
	}
	p, ok := cfg.Providers[r.Provider]
	if !ok {
		return
	}
	cwd, _ := os.Getwd()
	sys := PromptFor(cfg.SystemPrompt(), ModeInspect)
	if info := ContextProjecte(cwd); info != "" {
		sys += "\n\n" + info
	}
	SetDelegateRunner(func(ctx context.Context, prompt string) (string, error) {
		pol := &Policy{}
		hist := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: prompt}}
		for step := 1; step <= 12; step++ {
			if room := MakeRoom(ctx, client, cfg.PrimTarget(p, r), cfg.FallbackTarget(r), "", hist, Budget(cfg, r)); room.Changed() {
				hist = room.Hist
			}
			content, calls, _, err := client.ChatWithToolsFO(ctx, cfg.PrimTarget(p, r), cfg.FallbackTarget(r), hist, r.Temperature, r.MaxTokens, Specs(), nil)
			if err != nil {
				return "", fmt.Errorf("pas %d: %w", step, err)
			}
			hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
			if len(calls) == 0 {
				return content, nil
			}
			type resultat struct {
				out  string
				imgs []string
			}
			plans := PlanCalls(pol, ModeInspect, nil, calls)
			res := RunCalls(calls, func(i int) bool { return plans[i].Fixed != "" || plans[i].Ask }, func(_ int, c llm.ToolCall) resultat {
				o, oi, err := Exec(c.Function.Name, c.Function.Arguments)
				if err != nil {
					return resultat{out: "ERROR: " + err.Error()}
				}
				return resultat{out: o, imgs: oi}
			})
			for i, c := range calls {
				if plans[i].Fixed != "" || plans[i].Ask {
					hist = append(hist, ToolMsg(c, "EINA NO DISPONIBLE (subagent read-only): "+plans[i].Reason))
					continue
				}
				hist = append(hist, ToolMsg(c, res[i].out, res[i].imgs...))
			}
		}
		return "", fmt.Errorf("límit de 12 passos exhaurit")
	})
}

// delegateExec valida, llança en paral·lel i fusiona els resums.
func delegateExec(argsJSON string) (string, error) {
	var a struct {
		Tasks []struct {
			Prompt string `json:"prompt"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("arguments il·legibles: %w", err)
	}
	if len(a.Tasks) == 0 {
		return "", fmt.Errorf("cal almenys 1 tasca")
	}
	if len(a.Tasks) > MaxDelegateTasks {
		return "", fmt.Errorf("màxim %d subagents per crida (demanats %d)", MaxDelegateTasks, len(a.Tasks))
	}
	for i, t := range a.Tasks {
		if strings.TrimSpace(t.Prompt) == "" {
			return "", fmt.Errorf("tasca %d sense prompt", i+1)
		}
		if len(t.Prompt) > 2000 {
			return "", fmt.Errorf("tasca %d massa llarga (màx 2000 caràcters)", i+1)
		}
	}
	if delegateRunner == nil {
		return "", fmt.Errorf("subagents no disponibles en aquest loop")
	}
	outs := make([]string, len(a.Tasks))
	var wg sync.WaitGroup
	for i, t := range a.Tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
			defer cancel()
			out, err := delegateRunner(ctx, t.Prompt)
			if err != nil {
				outs[i] = fmt.Sprintf("== subagent %d: ERROR: %s", i+1, strings.TrimSpace(err.Error()))
				return
			}
			if strings.TrimSpace(out) == "" {
				out = "(sense resposta)"
			}
			outs[i] = fmt.Sprintf("== subagent %d:\n%s", i+1, strings.TrimSpace(out))
		}()
	}
	wg.Wait()
	return strings.Join(outs, "\n\n"), nil
}
