package agent

import "gregal/internal/config"

// PlanSystem obliga exploració read-only i tanca amb pla numerat +
// verificació. Contracte compartit del /plan (TUI, webapp, Telegram).
// La línia final és VERIFICATION (abans VERIFICACIÓ; TodosDelPla salta
// totes dues).
const PlanSystem = `PLAN MODE: you explore, you do not act.
- Read-only: read, grep, glob, web_search, web_fetch. NEVER call write or edit; if they would be needed, note it in the plan as "not available while exploring".
- Do not ask for confirmation: continue with what you find.
- When it is clear, reply ONLY with the plan: numbered steps (file + concrete change in each step) and a final line "VERIFICATION: <how to check it>".
Reply with the plan only, no preamble.`

// PlanBrief munta la consigna d'exploració amb memòria del projecte. Va
// com a missatge d'usuari (el loop read-only no té system), així que la
// directiva d'idioma la tanca.
func PlanBrief(cwd, task, lang string) string {
	brief := PlanSystem + "\n\nTask:\n" + task
	if info := ContextProjecte(cwd); info != "" {
		brief += "\n\n" + info
	}
	return brief + "\n\n" + config.LanguageDirective(lang)
}
