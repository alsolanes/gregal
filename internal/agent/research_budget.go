package agent

import (
	"fmt"

	"gregal/internal/llm"
)

// Pressupost de recerca web d'un torn.
//
// Vist el 2026-10-28 (docs/research/speed-local-models.md): «omple aquest PDF
// amb el document i completa-ho buscant a internet», amb el qwen flash local
// (~38 tokens/s, 15-40 s per pas), va gastar 8 dels 10 passos cercant i
// tornant a obrir pàgines per afinar detalls (la xifra exacta del padró, el
// número d'article d'una directiva) i no va arribar a escriure el PDF. En
// mode autònom, amb ampliacions fins a 2000 eines i 600 minuts, el mateix
// comportament no s'atura mai: és el torn de més de 70 minuts sense resposta.
//
// Regla: a ResearchNudgeAt crides web el model rep una guia (una vegada):
// prou recerca, fes l'entregable amb el que tens i marca el que no estigui
// confirmat. A ResearchCapAt les crides web es responen sense executar-se,
// amb el mateix missatge. Les eines que no són web no es toquen.
const (
	ResearchNudgeAt = 8
	ResearchCapAt   = 14
)

// esCridaWeb diu si l'eina és recerca a internet (cerca o lectura).
func esCridaWeb(name string) bool {
	switch name {
	case "web_search", "web_fetch", "web_images":
		return true
	}
	return false
}

// GuiaPressupostRecerca és la guia a ResearchNudgeAt crides web.
func GuiaPressupostRecerca(n int) string {
	return fmt.Sprintf("RESEARCH BUDGET: you have already made %d web searches/fetches in this turn. Stop researching: with what you already have, do the deliverable the user asked for NOW (write or edit the file, fill in the form, give the answer). Where a detail is not confirmed, say so in the result instead of searching more. At most %d more web calls are allowed in this turn, only for something the deliverable cannot do without.", n, ResearchCapAt-n)
}

// RecercaEsgotada és el resultat d'una crida web per sobre de ResearchCapAt.
func RecercaEsgotada(n int) string {
	return fmt.Sprintf("NOT EXECUTED: the web research budget of this turn is spent (%d web calls). Do not search or fetch again: finish the deliverable with the information you already have and mark what you could not confirm.", n)
}

// recercaTallada separa les crides web que ja no caben al pressupost.
// Les que sí hi caben compten des d'ara, perquè un sol pas amb deu cerques
// no passi de llarg del sostre.
func (t *Torn) recercaTallada(calls []llm.ToolCall) map[string]bool {
	var tallades map[string]bool
	for _, c := range calls {
		if !esCridaWeb(c.Function.Name) {
			continue
		}
		if t.cridesWeb >= ResearchCapAt {
			if tallades == nil {
				tallades = map[string]bool{}
			}
			tallades[c.ID] = true
			continue
		}
		t.cridesWeb++
	}
	if t.cridesWeb >= ResearchNudgeAt && !t.avisRecerca {
		t.avisRecerca = true
		t.guies = append(t.guies, GuiaPressupostRecerca(t.cridesWeb))
	}
	return tallades
}
