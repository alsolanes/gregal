package agent

// Peces compartides del mode autònom. El loop principal continua sent el
// mateix per no duplicar permisos, compactació ni recuperació d'errors; aquí
// només hi ha els límits i els checkpoints que fan possible una tasca llarga.

import (
	"context"
	"fmt"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
	"gregal/internal/verify"
)

// AutonomousCheck executa una comprovació declarada al contracte.
type AutonomousCheck struct {
	Command string `json:"command"`
	Code    int    `json:"code"`
	Output  string `json:"output,omitempty"`
}

// RunAutonomousCheck corre una comanda amb el mateix shell i entorn que el
// gate --verify. És exportada perquè TUI, web i headless comparteixin el
// comportament de verificació.
func RunAutonomousCheck(ctx context.Context, workdir, command string) AutonomousCheck {
	code, output := execVerificacio(ctx, workdir, command)
	return AutonomousCheck{Command: command, Code: code, Output: output}
}

// RunAutonomousChecks corre totes les comprovacions en ordre i s'atura en la
// primera fallada: la sortida serveix de guia per a la iteració següent.
func RunAutonomousChecks(ctx context.Context, workdir string, commands []string) []AutonomousCheck {
	var out []AutonomousCheck
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		check := RunAutonomousCheck(ctx, workdir, command)
		out = append(out, check)
		if check.Code != 0 {
			break
		}
	}
	return out
}

// AutonomousReview fa una revisió curta i independent del context recent i
// del diff. Si no hi ha un rol reviewer configurat, la revisió es considera
// omesa i la feina pot continuar: el gate mecànic continua tenint prioritat.
func AutonomousReview(ctx context.Context, client *llm.Client, cfg *config.Config, transcript, diff string) (verify.Verdict, string, error) {
	if client == nil || cfg == nil {
		return verify.Verdict{Approved: true, Summary: "revisió omesa"}, "", nil
	}
	a := cfg.AutonomousConfig()
	roleName := a.ReviewerRole
	r, ok := cfg.Roles[roleName]
	if !ok || strings.TrimSpace(r.Provider) == "" || strings.TrimSpace(r.Model) == "" {
		return verify.Verdict{Approved: true, Summary: "revisió omesa (sense rol reviewer)"}, "", nil
	}
	p, ok := cfg.Providers[r.Provider]
	if !ok {
		return verify.Verdict{}, "", fmt.Errorf("revisor: provider desconegut: %s", r.Provider)
	}
	return verify.Run(ctx, client, p, r, transcript, diff)
}

// RenderCheckpoint converteix les dades del checkpoint en una nota curta que
// es pot afegir a l'historial sense omplir-lo amb sortides senceres.
func RenderCheckpoint(checks []AutonomousCheck, verdict string) string {
	var b strings.Builder
	b.WriteString("CHECKPOINT AUTÒNOM:\n")
	if len(checks) == 0 {
		b.WriteString("- cap comprovació configurada\n")
	}
	for _, c := range checks {
		status := "PASSA"
		if c.Code != 0 {
			status = fmt.Sprintf("FALLA (codi %d)", c.Code)
		}
		fmt.Fprintf(&b, "- %s: %s", c.Command, status)
		if strings.TrimSpace(c.Output) != "" {
			b.WriteString("\n  " + strings.TrimSpace(c.Output))
		}
		b.WriteByte('\n')
	}
	if verdict != "" {
		b.WriteString("- revisió: " + verdict + "\n")
	}
	b.WriteString("Continua amb la següent fita; si una comprovació falla, arregla-la abans de donar la tasca per acabada.")
	return b.String()
}

// AutonomousDiff és una versió retallada del diff per al revisor de
// checkpoints. El revisor final continua rebent el diff complet.
func AutonomousDiff(dir string) string {
	d := tools.GitDiff(dir)
	if len([]rune(d)) <= 12000 {
		return d
	}
	r := []rune(d)
	return string(r[:12000]) + "\n…(diff retallat al checkpoint)"
}

// AutonomousTranscript retorna només els missatges textuals recents, sense
// abocar totes les sortides d'eines al prompt del revisor.
func AutonomousTranscript(hist []llm.Message) string {
	var rows []string
	for i := len(hist) - 1; i >= 0 && len(rows) < 8; i-- {
		m := hist[i]
		if (m.Role != "user" && m.Role != "assistant") || strings.TrimSpace(m.Content) == "" {
			continue
		}
		rows = append(rows, strings.TrimSpace(m.Role+": "+m.Content))
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return strings.Join(rows, "\n\n")
}

// AutonomousCost calcula el cost estimat del context actual si hi ha preus
// declarats per al provider/model. Retorna known=false per a models locals o
// sense preu, de manera que no inventa cap import.
func AutonomousCost(hist []llm.Message, provider, model string) (cost float64, known bool) {
	_, _, cost, known = HistCost(hist, provider, model)
	return cost, known
}
