package agent

import (
	"strings"

	"gregal/internal/llm"
)

// Recuperació quan la conversa no hi cap.
//
// El transport reintenta sol els errors passatgers (429/5xx/xarxa, vegeu
// llm.RetryAttempts), però un 400 de «context length exceeded» no és
// passatger: la mateixa petició tornarà a fallar sempre. Sense res més, un
// torn llarg (una tasca de codi amb vint passos d'eines i sortides de 8.000
// caràcters) moria a mig fer amb un error de proveïdor.
//
// Hermes Agent ho resol comprimint la conversa abans de la crida (>50% del
// context) i reintentant; el TUI de Gregal ja en té un resum. Aquí, al bucle
// de l'agent, hi ha la versió mínima que no pot perdre res essencial:
// s'escurça el contingut dels missatges antics (les sortides d'eines són el
// que ocupa) conservant SEMPRE l'ordre i el parell tool_call → tool, que els
// proveïdors validen, i es reintenta el mateix pas.

// màxim de retallades per torn: si després de dues encara no hi cap, el
// problema no és la mida i val més tancar amb un resum honest.
const maxRetallades = 2

// Retallat és el text que substitueix el contingut dels missatges antics.
const Retallat = "…(retallat per encabir la conversa al context del model)"

// EsErrorContext diu si un error vol dir que la petició no hi cap. Els
// proveïdors ho diuen de maneres diferents; totes parlen de mida o de
// finestra de context, mai de xarxa («context canceled» no hi entra).
func EsErrorContext(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{
		"context length", "context_length", "context window", "context_window",
		"maximum context", "too many tokens", "too long", "exceeds the maximum",
		"exceed context", "n_ctx", "max_tokens_exceeded", "input is too large",
		"reduce the length", "prompt is too long", "request too large",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// RetallaContext escurça la conversa perquè hi càpiga: conserva els últims
// keep missatges sencers, la primera consigna (retallada a maxLen) i el
// darrer missatge; de la resta en buida el contingut però manté el rol i les
// crides d'eina, perquè l'ordre i el parell tool_call → tool no es trenquin.
// Pura: testejable.
func RetallaContext(hist []llm.Message, keep, maxLen int) []llm.Message {
	if len(hist) == 0 {
		return hist
	}
	if keep < 2 {
		keep = 2
	}
	if maxLen <= 0 {
		maxLen = 600
	}
	out := make([]llm.Message, len(hist))
	copy(out, hist)
	// La primera consigna d'usuari és la tasca: no es pot perdre.
	for i := range out {
		if out[i].Role == "user" {
			if t := strings.TrimSpace(out[i].Content); t != "" && len(t) > maxLen {
				out[i].Content = t[:maxLen] + " […]"
			}
			break
		}
	}
	limit := len(out) - keep
	for i := 0; i < limit; i++ {
		// La consigna inicial ja s'ha conservat a dalt.
		if out[i].Role == "user" && i == 0 {
			continue
		}
		if out[i].Role != "tool" && out[i].Role != "assistant" {
			continue
		}
		if out[i].Content != "" && out[i].Content != Retallat {
			out[i].Content = Retallat
		}
	}
	return out
}

// contextRetallat aplica RetallaContext de manera progressiva: la primera
// vegada conserva 12 missatges, la segona només 6 (les eines del darrer pas).
func contextRetallat(hist []llm.Message, vegada int) []llm.Message {
	keep := 12
	if vegada >= 2 {
		keep = 6
	}
	return RetallaContext(hist, keep, 600)
}
