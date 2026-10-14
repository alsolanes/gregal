package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Verdict és el resultat d'una revisió.
type Verdict struct {
	Approved bool
	Summary  string
}

// BuildPrompt construeix el prompt del revisor amb rúbrica tancada.
// Exigeix un veredicte en primera línia per poder-lo parsejar. El prompt
// és en anglès i la línia del veredicte també (VERDICT: APPROVED / NEEDS
// REVISION); els motius surten en la llengua de la sessió (lang).
func BuildPrompt(transcript, diff, lang string) string {
	var b strings.Builder
	b.WriteString(`You are the project's senior code reviewer. Review the exchange and the diff.
Be short and direct. The FIRST line is MANDATORY and must be exactly one of:
  VERDICT: APPROVED
  VERDICT: NEEDS REVISION
Then 3-8 bullets with the reasons (bugs, scope, tests, security, over-engineering).
If there is nothing to object to, approve without filler.
ANTI-NOISE RULE: if the DIFF shows no changes and no tool was run, the
verdict is APPROVED with ONE summary line and no bullets. Never say
NEEDS REVISION while also saying there is nothing to validate: if there
is nothing to validate, it is APPROVED.

RUBRIC (in order):
1. Correctness: does it do what was asked? Edge cases?
2. Scope: does it touch only what it should? No opportunistic refactors?
3. Tests: is there evidence of verification (command + result)?
4. Security: secrets, injections, permissions, deletions?
5. Simplicity: a native/simple solution before a custom one?

`)
	b.WriteString(config.LanguageDirective(lang) + " Keep the VERDICT line exactly as written above.\n\nEXCHANGE:\n")
	b.WriteString(transcript)
	if strings.TrimSpace(diff) != "" {
		b.WriteString("\n\nDIFF:\n")
		b.WriteString(diff)
	} else {
		b.WriteString("\n\nDIFF: (no file changes in this turn)")
	}
	return b.String()
}

// ParseVerdict extreu el veredicte de la resposta del revisor. Accepta la
// línia en anglès (la del prompt actual) i la catalana d'abans; el revisor
// que no la traduïa bé tampoc no ha de quedar sense veredicte.
func ParseVerdict(text string) Verdict {
	up := strings.ToUpper(text)
	if strings.Contains(up, "VERDICT: NEEDS REVISION") || strings.Contains(up, "VEREDICTE: CAL REVISAR") {
		return Verdict{Approved: false, Summary: firstLines(text, 6)}
	}
	if strings.Contains(up, "VERDICT: APPROVED") || strings.Contains(up, "VEREDICTE: APROVAT") {
		return Verdict{Approved: true, Summary: firstLines(text, 3)}
	}
	// Sense veredicte explícit = no aprovat (estricte per defecte).
	return Verdict{Approved: false, Summary: "reviewer gave no explicit verdict:\n" + firstLines(text, 6)}
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// verifyLogPath és el JSONL de veredictes (D2). GREGAL_VERIFY_LOG el pot
// redirigir (tests); per defecte ~/.config/gregal/verify-log.jsonl.
func verifyLogPath() string {
	if p := os.Getenv("GREGAL_VERIFY_LOG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gregal", "verify-log.jsonl")
}

// logVerdict registra el veredicte (best-effort: mai no fa fallar la revisió).
func logVerdict(model string, v Verdict) {
	path := verifyLogPath()
	if path == "" {
		return
	}
	vs := "APROVAT"
	if !v.Approved {
		vs = "CAL REVISAR"
	}
	rec := map[string]string{
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"model":   model,
		"verdict": vs,
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, string(raw))
}

// DidRealWork diu si entre dos punts hi ha hagut "coses reals":
// escriptures al journal o diff git no buit. Pura (testeable).
func DidRealWork(journalBase, journalNow int, diff string) bool {
	return journalNow > journalBase || strings.TrimSpace(diff) != ""
}

// Run envia la transcripció al model revisor i retorna veredicte + text cru.
// lang és la llengua de la sessió (cfg.Lang()): la dels motius.
func Run(ctx context.Context, c *llm.Client, p config.Provider, r config.Role, transcript, diff, lang string) (Verdict, string, error) {
	raw, err := c.Chat(ctx, p.BaseURL, p.APIKey, r.Model,
		[]llm.Message{{Role: "user", Content: BuildPrompt(transcript, diff, lang)}},
		r.Temperature, r.MaxTokens)
	if err != nil {
		return Verdict{}, "", fmt.Errorf("revisor: %w", err)
	}
	v := ParseVerdict(raw)
	logVerdict(r.Model, v)
	return v, raw, nil
}
