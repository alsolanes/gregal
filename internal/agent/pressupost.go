package agent

import (
	"fmt"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Pressupost de passos que no deixa mai el torn buit.
//
// La tirada que ho va destapar (2026-10-16): l'usuari demana en mode Pla
// «revisa a fons el projecte buscant punts febles» i rep només
// «Plan · error: límit de 10 passos exhaurit». L'agent havia gastat els
// deu passos llegint fitxers i el bucle d'exploració (web, TUI, Telegram,
// subagents) tancava amb un error: tota la feina llegida, llençada. El
// motor dels torns normals ja feia una síntesi final; els bucles curts no.
//
// La regla, per a totes les superfícies i modes: quedar-se sense passos
// (o sense temps, o amb el proveïdor caigut a mig torn) acaba amb la
// millor resposta possible amb el que s'ha reunit, marcada com a parcial,
// i amb una manera de continuar. Tres peces:
//
//   - AvisPassosRestants: dos passos abans del final el model ho sap i pot
//     anar tancant, en comptes de topar amb el límit a mitja lectura.
//   - una crida final SENSE eines (PromptFinalExploracio o FinalPrompt):
//     resum, troballes i propers passos amb el que ja té.
//   - ResumDeFeina: si fins i tot la crida final falla, un resum mecànic de
//     l'historial (què s'ha mirat, l'últim que ha dit el model). Mai res.

// PassosAvis és a quants passos del final s'avisa el model.
const PassosAvis = 2

// PassosMinimsPla és el pressupost mínim d'una exploració de pla. Explorar
// és només lectura (barat i sense risc) i una revisió de projecte llegeix
// fàcilment vint fitxers: amb el max_steps: 10 que portava la plantilla del
// config, un pla de revisió no arribava mai a escriure's.
const PassosMinimsPla = 25

// PassosPla és el pressupost d'una exploració de pla: el de l'agent, però
// mai per sota de PassosMinimsPla.
func PassosPla(maxSteps int) int {
	return max(maxSteps, PassosMinimsPla)
}

// AvisPassosRestants és la guia que rep el model quan li queden pocs passos
// (restants inclou el que està a punt de fer).
func AvisPassosRestants(restants int) string {
	return fmt.Sprintf("STEP BUDGET: only %d step(s) left, including this one. Wrap up: use them only for what is essential, then give your final answer (plan, findings, next steps) without more tool calls. If the budget runs out first, you will have to answer from what you already have.", restants)
}

// AmbAvisPassos afegeix AvisPassosRestants a l'historial si el pas que va
// a començar (pas, 1-based) és dins dels últims PassosAvis del límit. Torna
// l'historial tal qual si no toca. Només un cop: el pas exacte.
func AmbAvisPassos(hist []llm.Message, pas, limit int) []llm.Message {
	restants := limit - pas + 1
	if limit <= PassosAvis || restants != PassosAvis || pas <= 1 {
		return hist
	}
	return AfegeixUsuari(hist, AvisPassosRestants(restants))
}

// AfegeixUsuari afegeix una consigna d'usuari a l'historial. Si l'últim
// missatge ja és d'usuari, s'hi enganxa: alguns proveïdors no accepten dos
// torns d'usuari seguits.
func AfegeixUsuari(hist []llm.Message, text string) []llm.Message {
	if n := len(hist); n > 0 && hist[n-1].Role == "user" {
		out := append([]llm.Message(nil), hist...)
		out[n-1].Content = strings.TrimRight(out[n-1].Content, "\n") + "\n\n" + text
		return out
	}
	return append(hist, llm.Message{Role: "user", Content: text})
}

// PromptFinalExploracio és la crida final d'una exploració (pla, revisió,
// subagent) quan s'acaba el pressupost: sense eines, la millor resposta
// amb el que s'ha reunit.
func PromptFinalExploracio(lang string) string {
	return "The step budget for this exploration is used up and tools are now disabled: do not call any tool and do not write tool-call markup. Give your best answer NOW from what you have already gathered, in the format the task asked for (a numbered plan with files and VERIFICATION if a plan was requested; findings with file references if a review was requested). Then add a short section listing what you could not check yet and the next steps to complete it. Do not apologise and do not describe your internal process.\n\n" + config.LanguageDirective(lang)
}

// PromptContinuaExploracio reprèn una exploració parcial amb pressupost nou
// i la conversa intacta.
const PromptContinuaExploracio = "Continue the exploration where you left off, with a fresh step budget. Do not re-read what you already read: focus on what your previous answer listed as not checked yet, then give the complete answer in the requested format."

// NetejaFinal treu de la resposta final qualsevol crida d'eina escrita com
// a text (la crida final va sense eines i res no s'executa).
func NetejaFinal(s string) string {
	net, _ := SenseEinaText(strings.TrimSpace(s))
	if n, dsml := llm.SenseCridesText(net); dsml {
		net = n
	}
	return strings.TrimSpace(net)
}

// motiuAmable tradueix el motiu d'interrupció a una frase curta i accionable.
// L'error cru del client duu la URL del proveïdor i el text de Go («provider
// https://…: dial tcp: lookup …»). A la conversa de l'usuari només hi ha
// d'arribar el que ha passat i què pot fer, mai adreces internes.
func motiuAmable(motiu, lang string) string {
	ca := lang == "ca"
	m := strings.ToLower(motiu)
	switch {
	case m == "" || strings.Contains(m, "passos"):
		return ""
	case strings.Contains(m, "no such host") || strings.Contains(m, "host not found"):
		if ca {
			return "no s'ha pogut arribar al proveïdor de models (DNS); comprova la connexió a internet"
		}
		return "could not reach the model provider (DNS); check the internet connection"
	case strings.Contains(m, "connection refused") || strings.Contains(m, "refused"):
		if ca {
			return "el proveïdor de models no accepta la connexió; comprova que el servidor local estigui engegat"
		}
		return "the model provider refused the connection; check that the local server is running"
	case strings.Contains(m, "deadline exceeded") || strings.Contains(m, "timeout") || strings.Contains(m, "context deadline"):
		if ca {
			return "el proveïdor de models ha trigat massa; torna-ho a provar d'aquí a una mica"
		}
		return "the model provider took too long; try again in a bit"
	case strings.Contains(m, "tls") || strings.Contains(m, "certificate"):
		if ca {
			return "error de certificat del proveïdor de models"
		}
		return "certificate error with the model provider"
	case strings.Contains(m, "context length") || strings.Contains(m, "maximum context"):
		if ca {
			return "la conversa ha superat la mida màxima del model"
		}
		return "the conversation exceeded the model context length"
	case strings.Contains(m, "unauthorized") || strings.Contains(m, "http 401") || strings.Contains(m, "invalid api key"):
		if ca {
			return "la clau del proveïdor de models no és vàlida (o ha caducat)"
		}
		return "the model provider key is invalid (or expired)"
	case strings.Contains(m, "http 429") || strings.Contains(m, "rate limit") || strings.Contains(m, "quota"):
		if ca {
			return "el proveïdor de models ha dit que hi ha massa tràfec; torna-ho a provar d'aquí a una mica"
		}
		return "the model provider is rate limiting; try again in a bit"
	case strings.Contains(m, "http 5"):
		if ca {
			return "el proveïdor de models té una incidència; torna-ho a provar d'aquí a una mica"
		}
		return "the model provider is having trouble; try again in a bit"
	case strings.Contains(m, "no side role"):
		return ""
	}
	// Qualsevol altre motiu: retallat net, sense URLs ni adreces internes.
	return retallaText(strings.Join(strings.Fields(motiu), " "), 120)
}

// ResumDeFeina és el resum mecànic de l'últim recurs: què s'ha consultat i
// l'últim text del model. Es fa servir quan la crida final també falla
// (proveïdor caigut, temps exhaurit): la feina feta no es perd.
// MotiuAmablePub és el pont públic de motiuAmable per als fronts (web,
// app): el motiu d'un torn aturat, en frase curta i sense adreces internes.
func MotiuAmablePub(motiu, lang string) string { return motiuAmable(motiu, lang) }

func ResumDeFeina(hist []llm.Message, motiu, lang string) string {
	ca := lang == "ca"
	motiu = motiuAmable(motiu, lang)
	var eines []string
	vistes := map[string]bool{}
	ultim := ""
	for _, m := range hist {
		if m.Role != "assistant" {
			continue
		}
		if t := strings.TrimSpace(m.Content); t != "" {
			ultim = t
		}
		for _, c := range m.ToolCalls {
			d := c.Function.Name
			if a := strings.TrimSpace(c.Function.Arguments); a != "" && a != "{}" {
				d += " " + retallaText(a, 80)
			}
			if !vistes[d] {
				vistes[d] = true
				eines = append(eines, d)
			}
		}
	}
	var b strings.Builder
	if ca {
		b.WriteString("Resposta parcial: el torn s'ha aturat abans de la resposta final")
	} else {
		b.WriteString("Partial answer: the turn stopped before the final answer")
	}
	if motiu != "" {
		b.WriteString(" (" + motiu + ")")
	}
	b.WriteString(".\n")
	if ultim != "" {
		if ca {
			b.WriteString("\nÚltim que havia dit el model:\n")
		} else {
			b.WriteString("\nLast thing the model said:\n")
		}
		b.WriteString(retallaRunes(ultim, 2000) + "\n")
	}
	if len(eines) > 0 {
		if ca {
			b.WriteString("\nEl que s'ha consultat:\n")
		} else {
			b.WriteString("\nWhat was looked at:\n")
		}
		const maxEines = 30
		for i, e := range eines {
			if i == maxEines {
				fmt.Fprintf(&b, "- … (+%d)\n", len(eines)-maxEines)
				break
			}
			b.WriteString("- " + e + "\n")
		}
	}
	if ca {
		b.WriteString("\nDigues «continua» per reprendre-ho amb la conversa intacta.")
	} else {
		b.WriteString("\nSay \"continue\" to resume with the conversation intact.")
	}
	return b.String()
}

// retallaRunes talla s a n runes conservant els salts de línia.
func retallaRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
