package flow

import (
	"context"
	"fmt"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// AgentRunner endolla el motor de grafs a l'agent de debò: els passos
// d'agent passen pel mateix loop que fa servir tota la resta, i els d'eina
// per les mateixes eines amb la mateixa política de permisos.
//
// Viu aquí i no a internal/agent perquè és flow qui depèn de l'agent, no a
// l'inrevés: el motor es continua podent provar amb un Runner de mentida.
type AgentRunner struct {
	Cfg    *config.Config
	Client *llm.Client
	// Policy decideix què pot fer un pas d'eina. Si és nil, un pas d'eina
	// no s'executa: preferim no fer res que fer-ho sense saber si es podia.
	Policy *agent.Policy
	// Mode per als passos d'agent que no en diuen cap.
	Mode string
	// AutoApprove passa les eines «ask» com a permeses. Un graf s'executa
	// sense ningú mirant; si no, cada pas es quedaria esperant.
	AutoApprove bool
	// Workspace fixa on es resolen lectures, escriptures i processos del graf.
	Workspace string
}

// Agent executa un pas d'agent.
func (a *AgentRunner) Agent(ctx context.Context, task, mode string, maxSteps int) (string, error) {
	if mode == "" {
		mode = a.Mode
	}
	if mode == "" {
		mode = "code"
	}
	if maxSteps <= 0 {
		maxSteps = a.Cfg.Agent.MaxSteps
	}
	res, err := agent.RunNonInteractiveIn(ctx, a.Client, a.Cfg, task, mode, maxSteps, a.AutoApprove, a.Workspace)
	if err != nil {
		return "", err
	}
	return res.Answer, nil
}

// Tool executa un pas d'eina, amb la política del projecte pel davant.
func (a *AgentRunner) Tool(ctx context.Context, name, argsJSON string) (string, error) {
	if strings.TrimSpace(argsJSON) == "" {
		argsJSON = "{}"
	}
	if a.Policy == nil {
		return "", fmt.Errorf("pas d'eina %q sense política de permisos: no s'executa", name)
	}
	mode := a.Mode
	if mode == "" {
		mode = "code"
	}
	dec, motiu := a.Policy.Decide(mode, name, argsJSON)
	if dec == "deny" {
		return "", fmt.Errorf("eina %q bloquejada: %s", name, motiu)
	}
	if dec == "ask" && !a.AutoApprove {
		return "", fmt.Errorf("eina %q demana permís i el flux corre sol: marca-la com a permesa o executa el flux amb aprovació automàtica", name)
	}
	out, _, err := agent.ExecIn("", a.Workspace, name, argsJSON)
	if err != nil {
		return "", err
	}
	return out, nil
}
