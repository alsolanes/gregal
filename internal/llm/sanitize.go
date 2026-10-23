package llm

import "strings"

// SanitizeHist garanteix que un històric sigui enviable a qualsevol
// proveïdor OpenAI-compatible. El cas real que ha portat aquesta funció:
// un torn que mor a mig pas (OOM del procés, cancel·lació, 400 de
// l'upstream) deixa a l'Hist un missatge d'assistant amb tool_calls la
// resposta del qual mai no s'hi ha afegit. Tornar a enviar aquella
// conversa fa que endpoints estrictes (opencode zen) responguin
// HTTP 400 {"model":"..."} — i com que un 400 no és recuperable, el torn
// mor una i altra vegada a cada missatge del usuari: la sessió queda
// enverinada per sempre.
//
// També retalla els raonaments antics: només el de l'últim missatge
// d'assistant pot ser exigit de tornada (zen thinking); els anteriors
// només inflen el context.
func SanitizeHist(msgs []Message) []Message {
	// Raonaments antics fora: només el de l'últim assistant es manté.
	últim := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			últim = i
			break
		}
	}
	if últim >= 0 {
		netos := append([]Message{}, msgs...)
		for i := range netos {
			if netos[i].Role == "assistant" && i != últim {
				netos[i].ReasoningContent = ""
			}
		}
		msgs = netos
	}
	// Primera passada: quins tool_call_id no tenen resposta.
	pend := map[string]bool{}
	for _, m := range msgs {
		if m.Role == "assistant" {
			for _, c := range m.ToolCalls {
				if c.ID != "" {
					pend[c.ID] = true
				}
			}
		}
		if m.Role == "tool" && m.ToolCallID != "" {
			delete(pend, m.ToolCallID)
		}
	}
	if len(pend) == 0 {
		return msgs
	}
	// Segona passada: aplanar els assistants amb crides pendents i treure
	// les respostes d'eina que quedin òrfenes (una resposta «tool» sense
	// crida és igual d'invàlida per a l'endpoint).
	res := make([]Message, 0, len(msgs))
	// aplanades: ids de crides aplanades → les seves respostes posteriors
	// també surten (una resposta «tool» sense crida és igual d'invàlida).
	aplanades := map[string]bool{}
	for _, m := range msgs {
		switch {
		case m.Role == "assistant" && len(m.ToolCalls) > 0 && tePendent(m, pend):
			for _, c := range m.ToolCalls {
				aplanades[c.ID] = true
			}
			nou := Message{Role: "assistant"}
			var b strings.Builder
			if strings.TrimSpace(m.Content) != "" {
				b.WriteString(m.Content)
				b.WriteString("\n\n")
			}
			b.WriteString("((la crida d'eina següent es va quedar sense resposta: ")
			noms := make([]string, 0, len(m.ToolCalls))
			for _, c := range m.ToolCalls {
				if c.ID != "" {
					noms = append(noms, c.Function.Name+"("+trunca120(c.Function.Arguments)+")")
				}
			}
			b.WriteString(strings.Join(noms, ", "))
			b.WriteString("))")
			nou.Content = b.String()
			res = append(res, nou)
		case m.Role == "tool" && aplanades[m.ToolCallID]:
			// Resposta d'una crida aplanada: fora (la crida ja no existeix).
		default:
			res = append(res, m)
		}
	}
	return res
}
func tePendent(m Message, pend map[string]bool) bool {
	for _, c := range m.ToolCalls {
		if pend[c.ID] {
			return true
		}
	}
	return false
}

func trunca120(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:120] + "…"
}
