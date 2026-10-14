package verify

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Puntuació d'un intent (best-of-N): el mateix revisor, amb una rúbrica
// tancada, dona una nota de 0 a 10 a un diff respecte a la tasca. Serveix
// per comparar intents entre ells, no per aprovar-ne cap.

// BuildScorePrompt munta la consigna de puntuació.
func BuildScorePrompt(task, diff, checks, lang string) string {
	if r := []rune(diff); len(r) > 20000 {
		diff = string(r[:20000]) + "\n…(diff truncated)"
	}
	var b strings.Builder
	b.WriteString(`You are the project's senior code reviewer scoring ONE candidate solution for a task.
The FIRST line is MANDATORY and must be exactly: SCORE: <integer 0-10>
Then at most 3 short bullets with the main reasons.

RUBRIC (weight in brackets):
1. Correctness [4]: does the diff do what the task asks, edge cases included?
2. Verification [2]: tests added or updated where it matters; the checks below pass.
3. Scope [2]: only what the task needs, no opportunistic refactors.
4. Simplicity and safety [2]: the simplest sound change; no secrets, no destructive operations.
An empty diff scores 0 unless the task needed no change.

`)
	b.WriteString(config.LanguageDirective(lang) + " Keep the SCORE line exactly as written above.\n\nTASK:\n")
	b.WriteString(strings.TrimSpace(task))
	if strings.TrimSpace(checks) != "" {
		b.WriteString("\n\nCHECKS:\n" + strings.TrimSpace(checks))
	}
	if strings.TrimSpace(diff) == "" {
		b.WriteString("\n\nDIFF: (empty)")
	} else {
		b.WriteString("\n\nDIFF:\n" + diff)
	}
	return b.String()
}

var reScore = regexp.MustCompile(`(?i)SCORE\s*[:=]\s*(\d{1,2})(?:\s*/\s*10)?`)

// ParseScore extreu la nota (0-10).
func ParseScore(text string) (int, bool) {
	m := reScore.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 || n > 10 {
		return 0, false
	}
	return n, true
}

// Score demana la nota al revisor. Torna la nota, les raons i l'error.
func Score(ctx context.Context, c *llm.Client, p config.Provider, r config.Role, task, diff, checks, lang string) (int, string, error) {
	raw, err := c.Chat(ctx, p.BaseURL, p.APIKey, r.Model,
		[]llm.Message{{Role: "user", Content: BuildScorePrompt(task, diff, checks, lang)}},
		r.Temperature, r.MaxTokens)
	if err != nil {
		return 0, "", fmt.Errorf("revisor: %w", err)
	}
	n, ok := ParseScore(raw)
	if !ok {
		return 0, firstLines(raw, 4), fmt.Errorf("revisor: sense línia SCORE")
	}
	return n, firstLines(raw, 4), nil
}
