package agent

// Crític del pla: un pas del rol reviewer que llegeix el pla (el del flux
// /plan o el contracte de fites del mode autònom) contra la tasca i el mapa
// del projecte i en treu els forats: passos que falten, passos arriscats i
// suposicions que ningú no ha comprovat. No reescriu res: torna problemes.
// El flux de pla els ensenya a la targeta; el mode autònom els passa al
// model perquè revisi el pla una vegada abans d'executar-lo.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// PlanIssue és un problema que el crític ha trobat al pla.
type PlanIssue struct {
	// Kind: missing | risky | assumption | other.
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// PlanCritique és el resultat del crític.
type PlanCritique struct {
	Issues []PlanIssue `json:"issues"`
	Model  string      `json:"model,omitempty"`
	// Skipped explica per què no s'ha fet (sense rol reviewer, error...).
	Skipped string `json:"skipped,omitempty"`
}

// CriticaPlaFunc és el ganxo del motor per al contracte autònom.
type CriticaPlaFunc func(ctx context.Context, task, plan string) (PlanCritique, error)

// BuildCriticPrompt munta la consigna del crític.
func BuildCriticPrompt(task, plan, repoMap, lang string) string {
	var b strings.Builder
	b.WriteString(`You are a senior engineer critiquing an implementation plan BEFORE it is executed.
Check the plan against the task and the repository map. Look only for real problems:
- MISSING: a step the task needs that the plan does not have (including tests or verification).
- RISKY: a step that could break things, lose data, touch the wrong files or is irreversible.
- ASSUMPTION: something the plan takes for granted that nobody has checked (a file, API, behaviour, dependency).
Reply with one issue per line, each starting with exactly MISSING:, RISKY: or ASSUMPTION:, at most 6 lines, most important first.
If the plan is sound, reply with exactly: NONE
No preamble, no praise, no rewritten plan.
`)
	b.WriteString(config.LanguageDirective(lang) + " Keep the MISSING:/RISKY:/ASSUMPTION:/NONE markers in English.\n\nTASK:\n")
	b.WriteString(strings.TrimSpace(task))
	b.WriteString("\n\nPLAN:\n")
	b.WriteString(strings.TrimSpace(plan))
	if m := strings.TrimSpace(repoMap); m != "" {
		if r := []rune(m); len(r) > 8000 {
			m = string(r[:8000]) + "\n…(map truncated)"
		}
		b.WriteString("\n\nREPOSITORY MAP:\n")
		b.WriteString(m)
	}
	return b.String()
}

var reIssue = regexp.MustCompile(`(?i)^\s*(?:[-*•]|\d+[.)])?\s*\**\s*(MISSING|RISKY|ASSUMPTION|UNTESTED ASSUMPTION)\s*\**\s*[:\-–]\s*(.+)$`)

// ParseCritique extreu els problemes de la resposta del crític.
func ParseCritique(text string) PlanCritique {
	c := PlanCritique{Issues: []PlanIssue{}}
	for _, l := range strings.Split(text, "\n") {
		m := reIssue.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		kind := strings.ToLower(m[1])
		if strings.Contains(kind, "assumption") {
			kind = "assumption"
		}
		t := strings.TrimSpace(strings.Trim(m[2], "* "))
		if t == "" {
			continue
		}
		c.Issues = append(c.Issues, PlanIssue{Kind: kind, Text: t})
		if len(c.Issues) >= 6 {
			break
		}
	}
	return c
}

// CriticRole torna el rol del crític: el reviewer del mode autònom
// (agent.autonomous.reviewer_role, per defecte «reviewer»).
func CriticRole(cfg *config.Config) (config.Provider, config.Role, bool) {
	if cfg == nil {
		return config.Provider{}, config.Role{}, false
	}
	r, ok := cfg.Roles[cfg.AutonomousConfig().ReviewerRole]
	if !ok || strings.TrimSpace(r.Provider) == "" || strings.TrimSpace(r.Model) == "" {
		return config.Provider{}, config.Role{}, false
	}
	p, ok := cfg.Providers[r.Provider]
	if !ok {
		return config.Provider{}, config.Role{}, false
	}
	return p, r, true
}

// CriticaPla fa la passada del crític. Sense rol reviewer, Skipped.
func CriticaPla(ctx context.Context, client *llm.Client, cfg *config.Config, task, plan, dir string) (PlanCritique, error) {
	p, r, ok := CriticRole(cfg)
	if !ok || client == nil {
		return PlanCritique{Issues: []PlanIssue{}, Skipped: "no reviewer role"}, nil
	}
	mapa := ""
	if strings.TrimSpace(dir) != "" {
		mapa = MapaProjecte(dir, task, nil)
	}
	cctx, cancel := context.WithTimeout(llm.WithSideCall(ctx), 90*time.Second)
	defer cancel()
	out, _, err := client.ChatFO(cctx, cfg.PrimTarget(p, r), cfg.FallbackTarget(r),
		[]llm.Message{{Role: "user", Content: BuildCriticPrompt(task, plan, mapa, cfg.Lang())}}, r.Temperature, r.MaxTokens, nil)
	if err != nil {
		return PlanCritique{Issues: []PlanIssue{}, Skipped: "critic failed: " + err.Error()}, err
	}
	c := ParseCritique(out)
	c.Model = r.Provider + "/" + r.Model
	return c, nil
}

// CriticaPlaHook torna el ganxo del motor (nil amb plan.critic: off).
func CriticaPlaHook(client *llm.Client, cfg *config.Config, dir string) CriticaPlaFunc {
	if client == nil || cfg == nil || !cfg.PlanCriticOn() {
		return nil
	}
	if _, _, ok := CriticRole(cfg); !ok {
		return nil
	}
	return func(ctx context.Context, task, plan string) (PlanCritique, error) {
		return CriticaPla(ctx, client, cfg, task, plan, dir)
	}
}

// RenderCritique és el text dels problemes per al model o per a la
// pantalla («- [missing] …»).
func RenderCritique(c PlanCritique) string {
	var b strings.Builder
	for _, i := range c.Issues {
		fmt.Fprintf(&b, "- [%s] %s\n", i.Kind, i.Text)
	}
	return strings.TrimRight(b.String(), "\n")
}

// GuiaCriticaContracte és la guia del mode autònom quan el crític troba
// problemes al contracte de fites.
func GuiaCriticaContracte(c PlanCritique) string {
	return "PLAN CRITIC: a reviewer checked your milestone plan against the task and the repository and raised these concerns:\n" +
		RenderCritique(c) +
		"\nRevise the plan ONCE now: update the checklist with todowrite to address the concerns that are valid (add missing steps, de-risk risky ones, verify assumptions first), then continue executing. Ignore concerns that are wrong, briefly saying why."
}
