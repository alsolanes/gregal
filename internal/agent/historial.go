package agent

import (
	"strings"
	"sync"

	"gregal/internal/llm"
)

// Historial on només s'afegeix (P2 de docs/research/harness-techniques.md).
//
// Un servidor local (llama.cpp, Ollama, LM Studio) reaprofita el KV cache
// fins al primer token que canvia: si un torn nou reescriu un missatge
// vell, es torna a processar tota la conversa des d'allà, i en un portàtil
// són desenes de segons abans del primer token. Per això:
//   - els missatges vells no es toquen a cada torn: el mapa del projecte
//     nou és un delta respecte de l'anterior (MapaProjecte) i els mapes
//     vells només es treuen, tots de cop, en compactar (SenseMapesAntics);
//   - el resum de compactació no viu al system prompt sinó en un missatge
//     d'usuari marcat (MissatgeResum), al davant de la cua que queda;
//   - el system prompt queda congelat durant la sessió (PrefixEstable): si
//     la memòria del projecte o l'índex de skills canvien a mig camí, el
//     canvi arriba com a nota al final del missatge nou.

// SenseMapesAntics torna una còpia de l'historial sense el mapa del
// projecte (MarcaContext, els fitxers adjunts que hi van i les notes de
// context) als missatges d'usuari. NOMÉS s'ha de fer servir en compactar:
// reescriure els missatges vells a cada torn invalidava tota la caché de
// prefix. Després, el pròxim torn torna a posar un mapa sencer (no en
// queda cap a l'historial). L'historial original no es modifica.
func SenseMapesAntics(hist []llm.Message) []llm.Message {
	out := make([]llm.Message, len(hist))
	copy(out, hist)
	for i := range out {
		if out[i].Role == "user" {
			out[i].Content = SenseMapa(out[i].Content)
		}
	}
	return out
}

// TeConversa diu si l'historial ja té alguna resposta de l'assistent: a
// partir d'aquí, un «ok» és la resposta a una pregunta (SmallTalkFor).
func TeConversa(hist []llm.Message) bool {
	for _, m := range hist {
		if m.Role == "assistant" {
			return true
		}
	}
	return false
}

// MarcaResum obre el missatge d'usuari que porta el resum de compactació
// (com DeepSeek). Al system prompt, cada compactació canviava el prefix
// sencer; aquí només canvia el principi de la conversa, que la compactació
// ja reescriu igualment.
const (
	MarcaResum      = "<compacted-summary>"
	MarcaResumFinal = "</compacted-summary>"
)

// MissatgeResum és el missatge que porta el resum de la conversa anterior.
func MissatgeResum(resum string) llm.Message {
	return llm.Message{Role: "user", Content: MarcaResum + "\n" +
		"[Summary of the earlier conversation — continue with this context]\n" +
		strings.TrimSpace(resum) + "\n" + MarcaResumFinal}
}

// EsResum diu si un missatge és el del resum de compactació.
func EsResum(m llm.Message) bool {
	return m.Role == "user" && strings.HasPrefix(m.Content, MarcaResum)
}

// AmbResum torna la conversa amb el resum al davant (i cap altre resum a
// dins). Idempotent: aplicar-la dues vegades amb el mateix resum dona el
// mateix. Sense resum, la conversa queda com és (una superfície que no
// desa session.Compacted no l'ha d'esborrar). Serveix també per carregar
// sessions desades abans, que el tenien a part (session.Compacted).
func AmbResum(convo []llm.Message, resum string) []llm.Message {
	if strings.TrimSpace(resum) == "" {
		return append([]llm.Message(nil), convo...)
	}
	return append([]llm.Message{MissatgeResum(resum)}, senseResums(convo)...)
}

func senseResums(convo []llm.Message) []llm.Message {
	out := make([]llm.Message, 0, len(convo))
	for _, m := range convo {
		if !EsResum(m) {
			out = append(out, m)
		}
	}
	return out
}

// CompactaConvo és el que queda d'una conversa entre torns després de
// resumir-la: el resum, i els darrers keep missatges (amb tall segur, mai
// un resultat d'eina sense la seva crida) sense mapes, perquè el torn
// següent en posi un de sencer. És l'únic moment que es reescriuen
// missatges vells: la compactació ja trenca la caché igualment.
func CompactaConvo(convo []llm.Message, resum string, keep int) []llm.Message {
	sense := senseResums(convo)
	cua := sense[SafeCut(sense, keep):]
	return AmbResum(SenseMapesAntics(cua), resum)
}

// MarcaNovetatsContext encapçala la nota que porta el context del projecte
// (AGENTS.md, memòria, skills) quan ha canviat a mitja sessió.
const MarcaNovetatsContext = "[project context updated"

// PrefixEstable congela el system prompt d'una sessió. La part que depèn
// del mode, l'idioma o el workspace (base) tria el prompt; la que depèn
// dels fitxers del projecte (ctx: AGENTS.md, memòria, índex de skills i de
// subagents) no el canvia: si canvia, Novetats la torna perquè vagi al
// final del missatge nou, i el prefix (system + historial) queda igual
// byte a byte. Cada base té el seu prompt congelat: una sessió que alterna
// modes (un torn de xat, un de codi) no el refà a cada canvi.
//
// El valor zero ja serveix; un punter nil no congela res (cada crida torna
// el prompt fresc), perquè els clients antics i els tests no hagin de fer
// res. És segur entre goroutines.
type PrefixEstable struct {
	mu      sync.Mutex
	congela map[string]*prefixCongelat
}

type prefixCongelat struct {
	ctx string // el que ja ha vist el model (al system o en una nota)
	sys string
}

const maxPrefixos = 8

// Sys torna el system prompt congelat per a aquesta base o, si encara no
// n'hi ha, build(ctx) (i el congela).
func (p *PrefixEstable) Sys(base, ctx string, build func(ctx string) string) string {
	if p == nil {
		return build(ctx)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.congela[base]; ok {
		return c.sys
	}
	if p.congela == nil || len(p.congela) >= maxPrefixos {
		p.congela = map[string]*prefixCongelat{}
	}
	c := &prefixCongelat{ctx: ctx, sys: build(ctx)}
	p.congela[base] = c
	return c.sys
}

// Novetats torna la nota per al missatge nou si ctx ha canviat respecte
// del que el model ja coneix amb aquesta base, i la dona per lliurada; ""
// si no hi ha res de nou o si encara no hi ha cap prompt congelat amb
// aquesta base (el pròxim Sys ja el durà fresc).
func (p *PrefixEstable) Novetats(base, ctx string) string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.congela[base]
	if !ok || c.ctx == ctx {
		return ""
	}
	c.ctx = ctx
	if strings.TrimSpace(ctx) == "" {
		return MarcaNovetatsContext + ": the project instructions, memory and skills given in the system prompt no longer exist; ignore them]"
	}
	return MarcaNovetatsContext + ": it replaces the one in the system prompt]\n" + ctx
}

// Reset fa que el pròxim Sys es torni a construir. Es crida en compactar
// (que ja reescriu el principi de la conversa: és el moment barat de posar
// el context nou al system) i en començar o reprendre una conversa.
func (p *PrefixEstable) Reset() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.congela = nil
	p.mu.Unlock()
}

// AmbNovetats enganxa la nota de context a la tasca (al final, on
// SenseMapa la treu per pintar-la).
func AmbNovetats(tasca, nota string) string {
	if nota == "" {
		return tasca
	}
	return tasca + "\n\n" + nota
}
