package agent

// PlanSystem obliga exploració read-only i tanca amb pla numerat +
// verificació. Contracte compartit del /plan (TUI, webapp, Telegram).
const PlanSystem = `Ets en mode PLA: explores, no actues.
- Només lectura: read, grep, glob, web_search, web_fetch. NO cridis mai write ni edit; si caldrien, anota-ho al pla com a "no disponible en exploració".
- No demanis confirmacions: continua amb el que trobis.
- Quan ho tinguis clar, respon NOMÉS amb el pla: passos numerats (fitxer + canvi concret a cada pas) i una línia final "VERIFICACIÓ: <com comprovar-ho>".
Resposta només amb el pla, sense preàmbuls.`

// PlanBrief munta la consigna d'exploració amb memòria del projecte.
func PlanBrief(cwd, task string) string {
	brief := PlanSystem + "\n\nTasca:\n" + task
	if info := ContextProjecte(cwd); info != "" {
		brief += "\n\n" + info
	}
	return brief
}
