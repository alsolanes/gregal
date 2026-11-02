package web

import "strings"

// tornText recull el que un vent escriu abans de cada fita del torn (una eina,
// un canvi de fase) i en treu les narracions que no formen part de la resposta
// final. Serveix perquè, quan l'app recarrega la conversa de l'historial, el
// que ja s'havia llegit no desaparegui: abans només es desava la resposta final
// i la narració es perdia.
type tornText struct {
	segs []string
	cur  strings.Builder
}

func (t *tornText) token(s string) { t.cur.WriteString(s) }

// fita tanca el tram obert. Es crida a cada canvi de fase del torn.
func (t *tornText) fita() {
	if s := strings.TrimSpace(t.cur.String()); s != "" {
		t.segs = append(t.segs, s)
	}
	t.cur.Reset()
}

// narracions retorna els trams que queden fora de la resposta final. Els que ja
// hi són (el darrer tram sol ser la mateixa resposta) no es repeteixen.
func (t *tornText) narracions(reply string) []string {
	t.fita()
	r := strings.TrimSpace(reply)
	out := make([]string, 0, len(t.segs))
	for _, s := range t.segs {
		if r != "" && strings.Contains(r, s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ambNarracions posa la narració davant del text final d'una veu (rotllana: el
// missatge és d'un sol vent, així que hi cap tot el que ha dit).
func ambNarracions(narracions []string, reply string) string {
	if len(narracions) == 0 {
		return reply
	}
	parts := append(append([]string(nil), narracions...), strings.TrimSpace(reply))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}
