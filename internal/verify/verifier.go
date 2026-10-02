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
// Exigeix un veredicte en primera línia per poder-lo parsejar.
func BuildPrompt(transcript, diff string) string {
	var b strings.Builder
	b.WriteString(`Ets el revisor sènior de codi del projecte. Revisa l'intercanvi i el diff.
Respon en català, curt i directe. Primera línia OBLIGATÒRIA, exacta:
  VEREDICTE: APROVAT
o bé
  VEREDICTE: CAL REVISAR
Després: 3-8 bullets amb el motiu (bugs, scope, tests, seguretat, sobreenginyeria).
Si no hi ha res a objectar, aprova sense palla.
REGLA ANTI-SOROLL: si el DIFF diu cap canvi i no consta cap eina
executada, el veredicte és APROVAT amb UNA sola línia de resum,
sense bullets. Mai diguis CAL REVISAR i alhora que no hi ha res
a validar: si no hi ha res a validar, és APROVAT.

RÚBRICA (per ordre):
1. Correctesa: fa el que es demana? Casos límit?
2. Scope: toca només el que toca? Sense refactors oportunistes?
3. Tests: hi ha evidència de verificació (comanda + resultat)?
4. Seguretat: secrets, injeccions, permisos, esborrats?
5. Simplicitat: solució nativa/simple abans que custom?

INTERCANVI:
`)
	b.WriteString(transcript)
	if strings.TrimSpace(diff) != "" {
		b.WriteString("\n\nDIFF:\n")
		b.WriteString(diff)
	} else {
		b.WriteString("\n\nDIFF: (cap canvi de fitxers en aquest torn)")
	}
	return b.String()
}

// ParseVerdict extreu el veredicte de la resposta del revisor.
func ParseVerdict(text string) Verdict {
	up := strings.ToUpper(text)
	if strings.Contains(up, "VEREDICTE: CAL REVISAR") {
		return Verdict{Approved: false, Summary: firstLines(text, 6)}
	}
	if strings.Contains(up, "VEREDICTE: APROVAT") {
		return Verdict{Approved: true, Summary: firstLines(text, 3)}
	}
	// Sense veredicte explícit = no aprovat (estricte per defecte).
	return Verdict{Approved: false, Summary: "revisor sense veredicte explícit:\n" + firstLines(text, 6)}
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
func Run(ctx context.Context, c *llm.Client, p config.Provider, r config.Role, transcript, diff string) (Verdict, string, error) {
	raw, err := c.Chat(ctx, p.BaseURL, p.APIKey, r.Model,
		[]llm.Message{{Role: "user", Content: BuildPrompt(transcript, diff)}},
		r.Temperature, r.MaxTokens)
	if err != nil {
		return Verdict{}, "", fmt.Errorf("revisor: %w", err)
	}
	v := ParseVerdict(raw)
	logVerdict(r.Model, v)
	return v, raw, nil
}
