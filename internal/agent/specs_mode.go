package agent

import (
	"strings"

	"gregal/internal/llm"
)

// Menys eines, millor tria. Un model petit amb 30+ esquemes d'eina falla
// més (un 8B quantitzat: 46 eines → falla, 19 → funciona) i cada esquema
// és prefill que es torna a processar quan la caché falla. El conjunt es
// decideix pel mode i el model (no canvia dins d'un torn, per no trencar
// la caché de KV).

// eines d'escriptura: en chat i inspect la política ja les denega; no
// cal ensenyar-les.
var einesEscriptura = map[string]bool{
	"write": true, "edit": true, "patch": true, "office_edit": true, "office_create": true,
	"git_commit": true, "gh_pr_create": true, "dashboard_save": true,
}

// einesCore és el conjunt compacte per a models locals petits: les eines
// d'un agent de codi i prou (com mini-swe-agent). Provat amb qwen3.5:9b a
// Ollama: amb les eines d'office, skill i web a la vista, per «crea
// stats.py» cridava office_create i després skill; amb aquestes, no.
var einesCore = map[string]bool{
	"read": true, "write": true, "edit": true, "patch": true, "bash": true, "grep": true, "glob": true,
	"question": true,
}

// FilterSpecs aplica mode i mode compacte a una llista d'eines.
func FilterSpecs(specs []llm.ToolSpec, mode string, compact bool) []llm.ToolSpec {
	readOnly := mode == ModeChat || mode == ModeInspect
	out := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if readOnly && einesEscriptura[s.Name] {
			continue
		}
		if compact && !einesCore[s.Name] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// SpecsFor són les eines (natives + delegate + MCP) per a un mode.
func SpecsFor(mode string, compact bool) []llm.ToolSpec {
	return FilterSpecs(SpecsAll(), mode, compact)
}

// compactCodeRules substitueix codeRules per a models locals petits: les
// mateixes idees en una fracció dels tokens (cada token del system prompt
// és prefill quan la caché falla).
const compactCodeRules = `
CODE MODE RULES (short)
1. Look before acting: read/grep/glob the files; never guess about the project.
2. Smallest change that fixes the cause. Keep names exactly as they are.
3. Small files (under ~300 lines): rewrite the whole file with write. Large files: edit/patch with the exact old text from read.
4. After editing, run the relevant check (tests/build) and fix what you broke.
5. Never claim something works without running it. Say what failed.
6. Finish with a short answer: what changed, how you verified it, what is pending.`

// PromptForModel és PromptFor amb la variant compacta per a models petits.
func PromptForModel(base, mode string, compact bool) string {
	p := PromptFor(base, mode)
	if compact && (mode == ModeCode || mode == "") {
		p = strings.Replace(p, codeRules, compactCodeRules, 1)
	}
	return p
}
