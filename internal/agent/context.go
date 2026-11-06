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
const Retallat = "…(truncated to fit the conversation in the model's context)"

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
// keep missatges sencers i, dels anteriors, escurça només les sortides
// d'eines i les respostes de l'assistent (a maxLen caràcters, o les buida del
// tot amb maxLen <= 0). El system i els missatges d'usuari (la tasca!) no es
// toquen mai. Manté el rol i les crides d'eina, perquè l'ordre i el parell
// tool_call → tool no es trenquin. Talla en runes, mai a mig accent. Pura:
// testejable.
func RetallaContext(hist []llm.Message, keep, maxLen int) []llm.Message {
	if len(hist) == 0 {
		return hist
	}
	if keep < 2 {
		keep = 2
	}
	out := make([]llm.Message, len(hist))
	copy(out, hist)
	limit := len(out) - keep
	for i := 0; i < limit; i++ {
		if out[i].Role != "tool" && out[i].Role != "assistant" {
			continue
		}
		c := out[i].Content
		if c == "" || c == Retallat || strings.HasSuffix(c, "\n"+Retallat) {
			continue
		}
		if maxLen <= 0 {
			out[i].Content = Retallat
			continue
		}
		// En runes: tallar bytes partia un accent i el proveïdor rebutjava l'UTF-8.
		if r := []rune(c); len(r) > maxLen {
			out[i].Content = string(r[:maxLen]) + "\n" + Retallat
		}
	}
	return out
}

// contextRetallat aplica RetallaContext de manera progressiva: la primera
// vegada conserva 12 missatges i escurça les sortides antigues a 400
// caràcters; la segona en conserva 6 (les eines del darrer pas) i buida la
// resta. La tasca no es retalla mai: amb el tall antic (600 bytes) es perdia
// el que demanava la persona just quan la conversa era més llarga.
func contextRetallat(hist []llm.Message, vegada int) []llm.Message {
	if vegada >= 2 {
		return RetallaContext(hist, 6, 0)
	}
	return RetallaContext(hist, 12, 400)
}
