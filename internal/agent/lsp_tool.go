package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/lsp"
	"gregal/internal/tools"
)

// L'eina `lsp`: navegació semàntica amb els servidors de llenguatge que hi
// hagi instal·lats (internal/lsp). Complementa symbols (definicions per
// nom, sense servidor) i grep (text): sap quina de les tres `Run` es crida
// en una línia, on es fa servir de veritat una funció, el tipus d'una
// expressió, i fa un rename que només toca el símbol, a tots els fitxers.

// SetupLSP aplica el config (lsp.mode: off, lsp.servers).
func SetupLSP(cfg *config.Config) {
	if cfg == nil {
		lsp.Configure(nil, false)
		return
	}
	var custom []lsp.Server
	for lang, s := range cfg.LSP.Servers {
		if len(s.Command) == 0 {
			continue
		}
		custom = append(custom, lsp.Server{Lang: lang, LanguageID: firstOf(s.LanguageID, lang), Commands: [][]string{s.Command}, Extensions: s.Extensions})
	}
	lsp.Configure(custom, strings.EqualFold(strings.TrimSpace(cfg.LSP.Mode), "off"))
}

// lspRule és la línia de regles que fa que el model triï `lsp` sol. L'eina
// ja era a la llista, però deepseek-v4.1-flash només la feia servir quan
// se li demanava (docs/research/next-improvements.md, 8.5): sense una
// frase que digui QUAN, tria grep. Només hi és si hi ha servidors de
// llenguatge, per no prometre una eina que no existeix; és constant per
// procés, i el prefix del prompt continua sent estable.
func lspRule(mode string) string {
	if _, ok := lspToolSpec(); !ok {
		return ""
	}
	switch mode {
	case ModeCode, ModeAutonomous, ModeDebug:
		return "\nLSP: for the uses of one specific function, method or type, where it is defined, or the type of an expression, call `lsp` (references, definition, hover) instead of grep: it is exact, across the whole project, and one call. To rename a symbol in several files use `lsp` rename, not edit. After editing, `lsp` diagnostics lists the compiler errors of the file."
	case ModeInspect, ModeChat:
		return "\nLSP: for the uses of one specific function, method or type, where it is defined, or the type of an expression, call `lsp` (references, definition, hover) instead of grep: it is exact, across the whole project, and one call."
	}
	return ""
}

func lspToolSpec() (llm.ToolSpec, bool) {
	langs := lsp.Llenguatges()
	if len(langs) == 0 {
		return llm.ToolSpec{}, false
	}
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{Name: "lsp", Description: "Ask the language server (" + strings.Join(langs, ", ") + ") about code, by position: give path, line (1-based, as read shows it) and symbol (the name on that line; or column). operation: definition (where it is defined), references (every place it is used, across the project), hover (type and docs), diagnostics (compiler errors and warnings of the file; no line needed), rename (renames the symbol everywhere it is used, with new_name; strings, comments and unrelated names with the same text are not touched). Prefer it to grep when you need the uses of one specific function, method or type, and to edit/patch for renaming a symbol across files.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"operation": map[string]any{"type": "string", "enum": []string{"definition", "references", "hover", "diagnostics", "rename"}},
			"path":      str,
			"line":      map[string]any{"type": "integer"},
			"symbol":    str,
			"column":    map[string]any{"type": "integer"},
			"new_name":  str,
		}, "required": []string{"operation", "path"}}}, true
}

type lspArgs struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Symbol    string `json:"symbol"`
	Column    int    `json:"column"`
	NewName   string `json:"new_name"`
}

func lspEsRename(argsJSON string) bool {
	var a lspArgs
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return strings.EqualFold(strings.TrimSpace(a.Operation), "rename")
}

func lspTool(ctx context.Context, dir string, seg *tools.Seguiment, argsJSON string) (string, error) {
	var a lspArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	arrel := procDirOr(dir)
	op := strings.ToLower(strings.TrimSpace(a.Operation))
	out, edits, err := lsp.Default.Do(ctx, arrel, lsp.Request{Op: op, Path: a.Path, Line: a.Line, Column: a.Column, Symbol: a.Symbol, NewName: a.NewName})
	if err != nil || op != "rename" || len(edits) == 0 {
		return out, err
	}
	// El rename s'escriu com qualsevol edició: foto al diari (desfer,
	// /rewind), seguiment de lectures, i el diff de cada fitxer.
	var b strings.Builder
	total := 0
	for _, e := range edits {
		total += e.Edits
	}
	fmt.Fprintf(&b, "renamed %q to %q: %d change(s) in %d file(s)\n", a.Symbol, a.NewName, total, len(edits))
	for _, e := range edits {
		resum, err := seg.Reescriu(e.Path, e.Old, e.New)
		if err != nil {
			return b.String(), err
		}
		b.WriteString(resum)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
