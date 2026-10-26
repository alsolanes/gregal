package agent

import (
	"regexp"
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

// einesOffice s'afegeixen al conjunt compacte quan la conversa va de
// documents d'Office (sense elles, un model petit no pot fer la tasca; amb
// elles sempre a la vista, les fa servir per a tot).
var einesOffice = map[string]bool{
	"office_read": true, "office_create": true, "office_edit": true, "office_render": true, "skill": true,
}

var officeRe = regexp.MustCompile(`(?i)\.(docx|dotx|docm|pptx|potx|pptm|xlsx|xltx|xlsm)\b|\b(word|excel|powerpoint|presentaci[oó]|diapositiv\w*|full de c[aà]lcul|spreadsheet|slides?|deck|workbook|llibre d'excel)\b`)

// OfficeAlHist diu si algun missatge de l'usuari parla de documents
// d'Office. Un cop surt, la resta de la conversa ho manté (prefix estable).
func OfficeAlHist(hist []llm.Message) bool {
	for _, m := range hist {
		if m.Role == "user" && officeRe.MatchString(m.Content) {
			return true
		}
	}
	return false
}

// FilterSpecs aplica mode i mode compacte a una llista d'eines. office
// afegeix les eines d'Office al conjunt compacte.
func FilterSpecs(specs []llm.ToolSpec, mode string, compact bool, office ...bool) []llm.ToolSpec {
	ambOffice := len(office) > 0 && office[0]
	readOnly := mode == ModeChat || mode == ModeInspect
	out := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if readOnly && einesEscriptura[s.Name] {
			continue
		}
		if compact && !einesCore[s.Name] && !(ambOffice && einesOffice[s.Name]) {
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

// SpecsForHist és SpecsFor tenint en compte si la conversa va d'Office.
func SpecsForHist(mode string, compact bool, hist []llm.Message) []llm.ToolSpec {
	return FilterSpecs(SpecsAll(), mode, compact, compact && OfficeAlHist(hist))
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
5a. When a step calls tools, start it with one short sentence saying what you are doing now.
6. Finish with a short answer: what changed, how you verified it, what is pending.`

// PromptForModel és PromptFor amb la variant compacta per a models petits.
func PromptForModel(base, mode string, compact bool) string {
	p := PromptFor(base, mode)
	if compact && (mode == ModeCode || mode == "") {
		p = strings.Replace(p, codeRules, compactCodeRules, 1)
	}
	return p
}
