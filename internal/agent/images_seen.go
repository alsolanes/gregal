package agent

import (
	"fmt"

	"gregal/internal/llm"
)

// DropSeenToolImages treu de l'historial les imatges de respostes d'eina
// que el model ja ha vist (n'hi ha una resposta de l'assistent després) i
// hi deixa una nota. Les imatges noves (encara sense resposta) i les que
// ha adjuntat l'usuari es queden.
//
// Vist en viu el 2026-10-28: pdf_read detail=images d'un PDF de 10 pàgines
// va afegir una imatge de 847 KB que s'enviava a cada pas següent. Amb un
// model local el prefill d'aquella imatge passava dels ~100 s de
// Cloudflare: tres HTTP 524 seguits (2 min cadascun) i el torn mort. El
// model ja havia descrit què hi veia; tornar-la a enviar no aportava res.
//
// No modifica h: torna una còpia quan cal canviar algun missatge.
func DropSeenToolImages(h []llm.Message) []llm.Message {
	lastAssistant := -1
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].Role == "assistant" {
			lastAssistant = i
			break
		}
	}
	var out []llm.Message
	for i, m := range h {
		if m.Role != "tool" || len(m.Images) == 0 || i > lastAssistant {
			continue
		}
		if out == nil {
			out = append([]llm.Message{}, h...)
		}
		m.Content += fmt.Sprintf("\n\n[%d imatge(s) d'aquesta eina ja vista(es) en un pas anterior: s'han tret de l'historial perquè no es tornin a enviar a cada pas. Si les tornes a necessitar, torna a cridar l'eina.]", len(m.Images))
		m.Images = nil
		out[i] = m
	}
	if out == nil {
		return h
	}
	return out
}
