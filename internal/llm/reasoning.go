package llm

import (
	"context"
	"strings"
	"sync"
)

// Raonament d'anada i tornada (reasoning_content).
//
// Els models amb mode de raonament que criden eines (DeepSeek V3.2+/V4,
// Kimi K2 thinking…) tornen el raonament al camp reasoning_content i
// exigeixen que, dins de la mateixa cadena de crides d'eina, el missatge
// assistant es torni a enviar amb aquell camp. Sense, alguns upstreams
// responen 400 «The `reasoning_content` in the thinking mode must be passed
// back to the API» i el torn es mor a mitja feina (vist en viu amb
// deepseek-v4.1-flash a opencode zen: intermitent, depèn de l'upstream).
//
// Les signatures de ChatWithTools* no tornen el raonament: canviar-les
// tocaria tots els fronts. En lloc d'això, qui condueix el torn posa un
// ReasoningCapture al context (WithReasoningCapture) i el llegeix després de
// la crida. Només es captura reasoning_content (el camp que s'ha de tornar);
// reasoning/thought/thinking d'altres proveïdors no es reenvien, perquè
// aquells proveïdors no ho demanen i un camp desconegut podria molestar.

// ReasoningCapture recull el reasoning_content de l'última resposta
// correcta d'una crida feta amb aquest context.
type ReasoningCapture struct {
	mu   sync.Mutex
	text string
}

type reasoningCaptureKey struct{}

// WithReasoningCapture adjunta un capturador de raonament al context.
func WithReasoningCapture(ctx context.Context, rc *ReasoningCapture) context.Context {
	if rc == nil {
		return ctx
	}
	return context.WithValue(ctx, reasoningCaptureKey{}, rc)
}

// Text torna el raonament capturat ("" si el proveïdor no n'ha enviat).
func (rc *ReasoningCapture) Text() string {
	if rc == nil {
		return ""
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.text
}

// Reset buida el capturador abans de reutilitzar-lo en una altra crida.
func (rc *ReasoningCapture) Reset() {
	if rc == nil {
		return
	}
	rc.mu.Lock()
	rc.text = ""
	rc.mu.Unlock()
}

// noteReasoning registra el reasoning_content d'una resposta acceptada: al
// capturador del context (si n'hi ha) i, si porta crides d'eina, a la
// memòria del client per id de crida.
func (c *Client) noteReasoning(ctx context.Context, calls []ToolCall, text string) {
	captureReasoning(ctx, text)
	if c != nil && text != "" && len(calls) > 0 {
		c.raonaments.recorda(calls, text)
	}
}

// maxRaonaments acota la memòria de raonaments (un per resposta amb crides).
const maxRaonaments = 512

// raonamentCache és la xarxa de seguretat per als bucles que construeixen
// el missatge assistant pel seu compte (subagents, pla, TUI, Telegram): si
// el missatge no porta reasoning_content però la seva primera crida és
// d'una resposta que sí que en tenia, el cable l'hi posa. Els ids de crida
// els genera el proveïdor i són únics; els de DSML («dsml-1»…) es
// repeteixen entre respostes i no es fan servir.
type raonamentCache struct {
	mu    sync.Mutex
	m     map[string]string
	ordre []string
}

func clauRaonament(calls []ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	id := calls[0].ID
	if id == "" || strings.HasPrefix(id, "dsml-") {
		return ""
	}
	return id
}

func (rc *raonamentCache) recorda(calls []ToolCall, text string) {
	k := clauRaonament(calls)
	if k == "" {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.m == nil {
		rc.m = map[string]string{}
	}
	if _, hi := rc.m[k]; !hi {
		rc.ordre = append(rc.ordre, k)
	}
	rc.m[k] = text
	for len(rc.ordre) > maxRaonaments {
		delete(rc.m, rc.ordre[0])
		rc.ordre = rc.ordre[1:]
	}
}

// omple torna els missatges amb el raonament recordat als assistant amb
// crides que no el porten. No modifica l'slice d'entrada; si no cal tocar
// res, el torna tal qual.
func (rc *raonamentCache) omple(msgs []Message) []Message {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if len(rc.m) == 0 {
		return msgs
	}
	var out []Message
	for i, m := range msgs {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 || m.ReasoningContent != "" {
			continue
		}
		text, ok := rc.m[clauRaonament(m.ToolCalls)]
		if !ok {
			continue
		}
		if out == nil {
			out = append([]Message(nil), msgs...)
		}
		out[i].ReasoningContent = text
	}
	if out == nil {
		return msgs
	}
	return out
}

// captureReasoning desa el reasoning_content d'una resposta acceptada.
func captureReasoning(ctx context.Context, text string) {
	rc, _ := ctx.Value(reasoningCaptureKey{}).(*ReasoningCapture)
	if rc == nil {
		return
	}
	rc.mu.Lock()
	rc.text = text
	rc.mu.Unlock()
}

// IsReasoningRoundTripError diu si el proveïdor ha rebutjat la petició
// perquè un missatge assistant no portava el reasoning_content que havia
// generat. Amb el raonament ja reenviat no hauria de passar; quan passa (un
// historial antic, un upstream estricte), un reintent sol anar bé: el motor
// el tracta com a recuperable una vegada.
func IsReasoningRoundTripError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "reasoning_content") &&
		(strings.Contains(msg, "passed back") || strings.Contains(msg, "must be"))
}

// normalitzaOrdreEines és una defensa del cable: els proveïdors estrictes
// (OpenAI, DeepSeek) rebutgen amb 400 qualsevol missatge que s'interposi
// entre un assistant amb tool_calls i els tool results que el responen
// («An assistant message with 'tool_calls' must be followed by tool
// messages responding to each 'tool_call_id'»). El motor no n'hauria de
// generar cap, però un sol avís mal col·locat matava el torn sencer. Aquí
// els missatges que no són tool i cauen enmig es mouen just després de
// l'últim resultat d'aquell lot. No toca res si l'ordre ja és correcte ni si
// algun resultat no arriba mai (llavors no hi ha on moure'ls).
func normalitzaOrdreEines(msgs []Message) []Message {
	if !calNormalitzar(msgs) {
		return msgs
	}
	out := make([]Message, 0, len(msgs))
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		out = append(out, m)
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		falten := map[string]bool{}
		for _, c := range m.ToolCalls {
			if c.ID != "" {
				falten[c.ID] = true
			}
		}
		// Busca fins on arriben els resultats d'aquest lot.
		fi := -1
		pendents := len(falten)
		for j := i + 1; j < len(msgs) && pendents > 0; j++ {
			r := msgs[j]
			if r.Role == "assistant" {
				break
			}
			if r.Role == "tool" && falten[r.ToolCallID] {
				falten[r.ToolCallID] = false
				pendents--
				fi = j
			}
		}
		if fi < 0 {
			continue
		}
		var eines, altres []Message
		for j := i + 1; j <= fi; j++ {
			if msgs[j].Role == "tool" {
				eines = append(eines, msgs[j])
			} else {
				altres = append(altres, msgs[j])
			}
		}
		out = append(out, eines...)
		out = append(out, altres...)
		i = fi
	}
	return out
}

// calNormalitzar evita la còpia en el cas normal (cap missatge enmig).
func calNormalitzar(msgs []Message) bool {
	for i, m := range msgs {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		falten := map[string]bool{}
		for _, c := range m.ToolCalls {
			if c.ID != "" {
				falten[c.ID] = true
			}
		}
		pendents := len(falten)
		vistAltre := false
		for j := i + 1; j < len(msgs) && pendents > 0; j++ {
			r := msgs[j]
			if r.Role == "assistant" {
				break
			}
			if r.Role == "tool" {
				if falten[r.ToolCallID] {
					falten[r.ToolCallID] = false
					pendents--
					if vistAltre {
						return true
					}
				}
				continue
			}
			vistAltre = true
		}
	}
	return false
}
