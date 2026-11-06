package agent

import "gregal/internal/config"

// PlanSystem obliga exploració read-only i tanca amb pla numerat +
// verificació. Contracte compartit del /plan (TUI, webapp, Telegram).
// La línia final és VERIFICATION (abans VERIFICACIÓ; TodosDelPla salta
// totes dues).
const PlanSystem = `PLAN MODE: you explore, you do not act.
- Read-only: read, grep, glob, web_search, web_fetch. NEVER call write or edit; if they would be needed, note it in the plan as "not available while exploring".
- Do not ask for confirmation: continue with what you find.
- Explore only what the plan needs: stop reading as soon as you know which files change and how.
- Keep the plan short enough to review in a minute (usually under 15 lines): what changes where and why, plus any decision the user may want to override. No full code and no test-by-test specs; the implementer works those out.
- When it is clear, reply ONLY with the plan: numbered steps (file + concrete change in each step) and a final line "VERIFICATION: <how to check it>".
Reply with the plan only, no preamble.`

// PlanSystemPrompt és el system del mode Pla quan la tasca va a part, com a
// missatge d'usuari: el model distingeix les regles de la feina (abans tot
// anava en un sol missatge d'usuari, i en retallar el context per mida
// la tasca es quedava en 600 bytes).
func PlanSystemPrompt(cwd, lang string) string {
	sys := PlanSystem + "\n- Shell commands are not available while exploring: use read, grep, glob and symbols on the code."
	if info := ContextProjecte(cwd); info != "" {
		sys += "\n\n" + info
	}
	return sys + "\n\n" + config.LanguageDirective(lang)
}

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
