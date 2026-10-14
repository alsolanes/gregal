package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Subagents definits per l'usuari: fitxers Markdown amb capçalera YAML a
// .gregal/agents/*.md (projecte) i ~/.config/gregal/agents/*.md (persona).
// La capçalera diu qui és (name, description), amb quin model treballa
// (model o role), quines eines pot fer servir (tools), si pot escriure
// (mode: inspect|code) i quants passos té (max_steps). El cos és el prompt
// del subagent. Al prompt del pare només hi va l'índex (nom i descripció):
// el cos arriba quan delegate el fa servir.

// SubagentDef és un subagent definit en un fitxer.
type SubagentDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Model       string   `json:"model,omitempty"`
	Role        string   `json:"role,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Mode        string   `json:"mode"`
	MaxSteps    int      `json:"max_steps"`
	Prompt      string   `json:"-"`
	// Source és "project" o "user"; Path, el fitxer d'on surt.
	Source   string   `json:"source"`
	Path     string   `json:"path"`
	Warnings []string `json:"warnings,omitempty"`
}

const (
	defaultSubagentSteps = 12
	maxSubagentSteps     = 40
	maxSubagentPrompt    = 8000
)

var subagentNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// subagentBannedTools són eines d'estat del pare o recursives: cap fill no
// les fa servir, encara que la capçalera les demani.
var subagentBannedTools = map[string]bool{"delegate": true, "question": true, "todowrite": true, "preview": true, "dashboard_save": true}

// DirsSubagents són els directoris de definicions, de menys a més
// específic: primer les de la persona, després les del projecte (que manen).
func DirsSubagents(cwd string) []string {
	var dirs []string
	if base := filepath.Dir(config.DefaultPath()); base != "" && base != "." {
		dirs = append(dirs, filepath.Join(base, "agents"))
	}
	if cwd = strings.TrimSpace(cwd); cwd != "" {
		dirs = append(dirs, filepath.Join(cwd, ".gregal", "agents"))
	}
	return dirs
}

// LoadSubagents llegeix les definicions del projecte i de la persona. Una
// del projecte tapa la de la persona que es digui igual. Els fitxers
// il·legibles o sense capçalera vàlida s'ignoren.
func LoadSubagents(cwd string) []SubagentDef {
	dirs := DirsSubagents(cwd)
	per := map[string]SubagentDef{}
	for i, dir := range dirs {
		source := "user"
		if strings.TrimSpace(cwd) != "" && i == len(dirs)-1 {
			source = "project"
		}
		rutes, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		sort.Strings(rutes)
		for _, ruta := range rutes {
			raw, err := os.ReadFile(ruta)
			if err != nil {
				continue
			}
			def, err := ParseSubagent(string(raw))
			if err != nil {
				continue
			}
			def.Source, def.Path = source, ruta
			per[def.Name] = def
		}
	}
	out := make([]SubagentDef, 0, len(per))
	for _, d := range per {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FindSubagent busca una definició pel nom (sense distingir majúscules).
func FindSubagent(defs []SubagentDef, name string) (SubagentDef, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, d := range defs {
		if d.Name == name {
			return d, true
		}
	}
	return SubagentDef{}, false
}

// SubagentNames són els noms, per als missatges d'error.
func SubagentNames(defs []SubagentDef) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Name)
	}
	return out
}

// ParseSubagent llegeix un fitxer de definició: capçalera YAML entre «---»
// i el cos com a prompt. Sense nom o descripció no és un subagent.
func ParseSubagent(raw string) (SubagentDef, error) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return SubagentDef{}, fmt.Errorf("missing YAML front matter")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return SubagentDef{}, fmt.Errorf("unterminated YAML front matter")
	}
	head := text[4 : 4+end]
	body := strings.TrimSpace(strings.TrimLeft(text[4+end+len("\n---"):], "-\n"))
	var fm struct {
		Name        string    `yaml:"name"`
		Description string    `yaml:"description"`
		Model       string    `yaml:"model"`
		Role        string    `yaml:"role"`
		Tools       yaml.Node `yaml:"tools"`
		Mode        string    `yaml:"mode"`
		MaxSteps    int       `yaml:"max_steps"`
	}
	if err := yaml.Unmarshal([]byte(head), &fm); err != nil {
		return SubagentDef{}, fmt.Errorf("invalid YAML front matter: %w", err)
	}
	def := SubagentDef{
		Name:        strings.ToLower(strings.TrimSpace(fm.Name)),
		Description: strings.Join(strings.Fields(fm.Description), " "),
		Model:       strings.TrimSpace(fm.Model),
		Role:        strings.TrimSpace(fm.Role),
		Mode:        strings.ToLower(strings.TrimSpace(fm.Mode)),
		MaxSteps:    fm.MaxSteps,
		Prompt:      body,
	}
	if !subagentNameRe.MatchString(def.Name) {
		return SubagentDef{}, fmt.Errorf("invalid name %q (lowercase letters, digits, - and _)", fm.Name)
	}
	if def.Description == "" {
		return SubagentDef{}, fmt.Errorf("subagent %q needs a description", def.Name)
	}
	if r := []rune(def.Description); len(r) > 300 {
		def.Description = string(r[:300])
	}
	switch def.Mode {
	case "":
		def.Mode = ModeInspect
	case ModeInspect, ModeCode:
	default:
		def.Warnings = append(def.Warnings, fmt.Sprintf("unknown mode %q: using inspect", def.Mode))
		def.Mode = ModeInspect
	}
	switch {
	case def.MaxSteps <= 0:
		def.MaxSteps = defaultSubagentSteps
	case def.MaxSteps > maxSubagentSteps:
		def.Warnings = append(def.Warnings, fmt.Sprintf("max_steps capped at %d", maxSubagentSteps))
		def.MaxSteps = maxSubagentSteps
	}
	if r := []rune(def.Prompt); len(r) > maxSubagentPrompt {
		def.Prompt = string(r[:maxSubagentPrompt])
		def.Warnings = append(def.Warnings, fmt.Sprintf("prompt truncated to %d characters", maxSubagentPrompt))
	}
	var requested []string
	switch fm.Tools.Kind {
	case yaml.ScalarNode:
		for _, t := range strings.Split(fm.Tools.Value, ",") {
			requested = append(requested, t)
		}
	case yaml.SequenceNode:
		for _, n := range fm.Tools.Content {
			requested = append(requested, n.Value)
		}
	}
	seen := map[string]bool{}
	for _, t := range requested {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		if !tools.EsNativa(t) || subagentBannedTools[t] {
			def.Warnings = append(def.Warnings, fmt.Sprintf("tool %q is not available to subagents", t))
			continue
		}
		def.Tools = append(def.Tools, t)
	}
	if len(requested) > 0 && len(def.Tools) == 0 {
		return SubagentDef{}, fmt.Errorf("subagent %q lists no usable tools", def.Name)
	}
	return def, nil
}

// IndexSubagents és el bloc compacte del prompt del pare: un subagent per
// línia. Buit si no n'hi ha cap.
func IndexSubagents(cwd string) string {
	return subagentIndex(LoadSubagents(cwd))
}

func subagentIndex(defs []SubagentDef) string {
	if len(defs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("SUBAGENTS (user-defined; to use one, pass its name as `agent` in a delegate task):\n")
	for _, d := range defs {
		fmt.Fprintf(&b, "- %s (%s): %s\n", d.Name, d.Mode, d.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}

// subagentAllowed diu si el subagent pot fer servir l'eina. Sense llista,
// totes les natives menys les d'estat del pare.
func subagentAllowed(def *SubagentDef, name string) bool {
	if subagentBannedTools[name] {
		return false
	}
	if def == nil || len(def.Tools) == 0 {
		return true
	}
	for _, t := range def.Tools {
		if t == name {
			return true
		}
	}
	return false
}

// filterSpecs deixa només les eines que el subagent pot fer servir.
func filterSpecs(specs []llm.ToolSpec, allowed func(string) bool) []llm.ToolSpec {
	out := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if allowed(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// SubagentRole resol el rol (provider, model, paràmetres) d'una definició
// a partir del rol base: role: tria un rol del config; model: canvia el
// model ("provider/model" o només el model del mateix provider).
func SubagentRole(cfg *config.Config, base config.Role, def *SubagentDef) (config.Role, error) {
	r := base
	if def == nil || cfg == nil {
		return r, nil
	}
	if def.Role != "" {
		rr, ok := cfg.Roles[def.Role]
		if !ok {
			return r, fmt.Errorf("subagent %q: role %q is not configured", def.Name, def.Role)
		}
		r = rr
	}
	if m := def.Model; m != "" {
		if prov, model, ok := strings.Cut(m, "/"); ok {
			if _, known := cfg.Providers[prov]; known {
				r.Provider, r.Model = prov, model
				return r, nil
			}
		}
		r.Model = m
	}
	return r, nil
}

// Escriptures dels subagents en mode code: només passen soles dins d'un
// worktree aïllat; fora d'un worktree cada acció que no és lectura passa
// per l'aprovació de l'usuari (el mateix flux que el pare). Sense ningú
// que aprovi, el subagent queda en només lectura.

type delegateApproverKey struct{}

// WithDelegateApprover instal·la qui aprova les accions dels subagents en
// mode code (la web hi penja el seu approve_request). fn pot ser cridada
// en paral·lel per diversos fills.
func WithDelegateApprover(ctx context.Context, fn func(ctx context.Context, call llm.ToolCall, reason string) bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, delegateApproverKey{}, fn)
}

func delegateApproverFrom(ctx context.Context) func(context.Context, llm.ToolCall, string) bool {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(delegateApproverKey{}).(func(context.Context, llm.ToolCall, string) bool)
	return fn
}

// IsolatedWorktree diu si el directori és dins d'un worktree de gregal
// (.gregal/worktrees/<nom>): allà les escriptures no toquen l'arbre
// principal i es poden revisar abans d'aplicar-les.
func IsolatedWorktree(dir string) bool {
	return strings.TrimSpace(dir) != "" && tools.WorktreeNameOf(dir) != ""
}

type subagentDefKey struct{}

func withSubagentDef(ctx context.Context, def *SubagentDef) context.Context {
	if def == nil {
		return ctx
	}
	return context.WithValue(ctx, subagentDefKey{}, def)
}

// SubagentFromContext torna la definició que fa servir aquest fill (nil =
// subagent genèric de lectura).
func SubagentFromContext(ctx context.Context) *SubagentDef {
	if ctx == nil {
		return nil
	}
	def, _ := ctx.Value(subagentDefKey{}).(*SubagentDef)
	return def
}

// delegateWorkspace és el directori on treballen els fills d'aquest torn.
func delegateWorkspace(ctx context.Context) string {
	if dir := delegateScopeFromContext(ctx).dir; dir != "" {
		return dir
	}
	wd, _ := os.Getwd()
	return wd
}
