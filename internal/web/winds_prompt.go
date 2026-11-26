package web

import (
	"context"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Missatges estructurats d'un torn de vent.
//
// Abans tot el torn era UN sol missatge d'usuari: la persona, les regles, la
// conversa anterior aplanada («USER: … ASSISTANT: …») i la pregunta. El model
// llegia la conversa com un document i no com a torns seus, les regles pesaven
// com a text de l'usuari i, quan el context no hi cabia, el retall del primer
// missatge d'usuari (600 caràcters) s'enduia la persona sencera. Ara és com
// als xats bons: system estable, torns reals i el missatge nou al final.

type windPromptKey struct{}

// withWindPrompt fa que el loop del vent comenci amb aquests missatges en
// lloc del brief pla (que continua servint de text per a tests i registres).
func withWindPrompt(ctx context.Context, msgs []llm.Message) context.Context {
	if len(msgs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, windPromptKey{}, msgs)
}

func windPromptOf(ctx context.Context) []llm.Message {
	msgs, _ := ctx.Value(windPromptKey{}).([]llm.Message)
	return msgs
}

const (
	windTurnsMaxMsgs  = 24
	windTurnsMaxChars = 14000
	windTurnMaxChars  = 3000
	// windStoreMaxMsgs és el sostre de l'historial PERSISTIT (disc + memòria):
	// el prompt de cada torn només llegeix els recents (windChatTurns), però
	// sense poda el fitxer creixia sense límit. Prou marge per al prompt (24)
	// més cua per no perdre el fil proper.
	windStoreMaxMsgs = 100
)

// podaHistorialVent retalla l'historial desat al sostre: conserva els N més
// recents i, si el més antic conservat és d'assistent, el descarta (comença
// sempre per usuari, com exigeixen alguns proveïdors).
func podaHistorialVent(history []llm.Message) []llm.Message {
	if len(history) <= windStoreMaxMsgs {
		return history
	}
	out := append([]llm.Message(nil), history[len(history)-windStoreMaxMsgs:]...)
	for len(out) > 0 && out[0].Role != "user" {
		out = out[1:]
	}
	return out
}

// windChatTurns converteix l'historial desat en torns reals: només usuari i
// assistent amb text, els més recents dins del pressupost, cada missatge
// retallat, torns seguits del mateix rol units (un avís del vent seguit d'una
// resposta seva) i començant sempre per l'usuari (alguns proveïdors ho exigeixen).
//
// Pensat perquè el prefix del torn següent sigui idèntic (memòria cau del
// proveïdor, sobretot amb un model local):
//   - cada missatge de l'usuari porta l'hora amb què es va enviar
//     ([time now: …], refeta del Ts, que és l'inici del torn), igual que
//     quan era el missatge nou: abans s'enviava amb hora i tornava sense, i
//     el prefix es trencava a cada torn;
//   - quan l'historial passa del pressupost no llisca un torn cada vegada:
//     retalla de cop windTurnsCutStep missatges i es queda quiet fins que hi
//     torna a passar (windTurnsStart).
func windChatTurns(previous []llm.Message) []llm.Message {
	type cand struct {
		m    llm.Message
		text string
	}
	var all []cand
	for _, m := range previous {
		if m.Role != "user" && m.Role != "assistant" || m.Kind == memoryKind || m.Kind == consultKind {
			continue
		}
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		// Un vent convidat (winds_invite.go): no ho va dir el vent d'aquest fil.
		if m.Kind == guestKind && m.SpeakerName != "" {
			text = "[" + m.SpeakerName + ", another wind the user invited into this chat, said:] " + text
		}
		if r := []rune(text); len(r) > windTurnMaxChars {
			text = string(r[:windTurnMaxChars]) + " […]"
		}
		if m.Role == "user" && m.Kind == "" && m.Ts > 0 {
			text += windClockTag(time.Unix(m.Ts, 0))
		}
		all = append(all, cand{m, text})
	}
	// Inici natural: els més recents que caben (com a mínim l'últim).
	start, total := len(all), 0
	for start > 0 && len(all)-start < windTurnsMaxMsgs {
		n := len(all[start-1].text)
		if total+n > windTurnsMaxChars && start < len(all) {
			break
		}
		total += n
		start--
	}
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = windTurnID(c.m, c.text)
	}
	start = windTurnsStart(ids, start)
	var out []llm.Message
	for _, c := range all[start:] {
		if n := len(out); n > 0 && out[n-1].Role == c.m.Role {
			out[n-1].Content += "\n\n" + c.text
			continue
		}
		out = append(out, llm.Message{Role: c.m.Role, Content: c.text})
	}
	if len(out) > 0 && out[0].Role == "assistant" {
		out = append([]llm.Message{{Role: "user", Content: "(earlier conversation)"}}, out...)
	}
	return out
}

// windTurnsCutStep és quants missatges es retallen de cop quan l'historial
// passa del pressupost (uns quatre torns): el prefix aguanta fins que hi torna
// a passar, en lloc de canviar a cada torn.
const windTurnsCutStep = 8

// windTurnsAnchors recorda on es va tallar l'historial: la identitat del
// primer missatge enviat (Ts, rol i contingut), no la seva posició, que es
// mou quan l'historial desat es poda (podaHistorialVent).
var (
	windTurnsAnchorMu sync.Mutex
	windTurnsAnchors  = map[string]time.Time{}
)

// windTurnsStart tria on comença l'historial enviat, a partir de l'inici
// natural (el primer que hi cap). Si cap tall anterior encara hi cap, es
// manté; si no, se'n fa un de nou windTurnsCutStep missatges més enllà.
func windTurnsStart(ids []string, natural int) int {
	if natural <= 0 {
		return 0
	}
	windTurnsAnchorMu.Lock()
	defer windTurnsAnchorMu.Unlock()
	for i := natural; i < len(ids) && i < natural+windTurnsCutStep; i++ {
		if _, ok := windTurnsAnchors[ids[i]]; ok {
			windTurnsAnchors[ids[i]] = time.Now()
			return i
		}
	}
	start := natural + windTurnsCutStep
	// Sempre en queden almenys els dos últims torns.
	if maxStart := len(ids) - 4; start > maxStart {
		start = max(natural, maxStart)
	}
	if len(windTurnsAnchors) > 2000 {
		for k, t := range windTurnsAnchors {
			if time.Since(t) > 24*time.Hour {
				delete(windTurnsAnchors, k)
			}
		}
	}
	windTurnsAnchors[ids[start]] = time.Now()
	return start
}

func windTurnID(m llm.Message, text string) string {
	h := fnv.New64a()
	h.Write([]byte(m.Role + "\x00" + text))
	return strconv.FormatInt(m.Ts, 10) + ":" + strconv.FormatUint(h.Sum64(), 36)
}

// windTurnMessages munta system + torns + missatge nou. Si l'últim torn desat
// ja és de l'usuari (un missatge sense resposta), s'hi enganxa el nou.
func windTurnMessages(system string, previous []llm.Message, msg string) []llm.Message {
	return windTurnMessagesAt(system, previous, msg, time.Now())
}

// windTurnMessagesAt és windTurnMessages amb l'hora del torn: la mateixa amb
// què es desa el missatge (Ts), perquè al torn següent torni idèntic.
func windTurnMessagesAt(system string, previous []llm.Message, msg string, now time.Time) []llm.Message {
	msgs := []llm.Message{{Role: "system", Content: system}}
	msg += windClockTag(now)
	turns := windChatTurns(previous)
	if n := len(turns); n > 0 && turns[n-1].Role == "user" {
		turns[n-1].Content += "\n\n" + msg
		return append(msgs, turns...)
	}
	return append(append(msgs, turns...), llm.Message{Role: "user", Content: msg})
}

// windContinueHistory és l'historial per a «Continua»: abans s'enviava la
// conversa desada sense cap system, i la continuació perdia la persona i
// les regles. Ara porta el mateix system que el torn normal.
func windContinueHistory(s *Server, wind companion.Wind, previous []llm.Message) []llm.Message {
	last := ""
	for i := len(previous) - 1; i >= 0; i-- {
		if previous[i].Role == "user" {
			last = previous[i].Content
			break
		}
	}
	turns := windChatTurns(previous)
	// La parcial que es reprèn va sencera: retallada, el model no sabria on parava.
	if n, m := len(turns), len(previous); n > 0 && m > 0 && turns[n-1].Role == "assistant" && previous[m-1].Role == "assistant" {
		turns[n-1].Content = strings.TrimSpace(previous[m-1].Content)
	}
	return append([]llm.Message{{Role: "system", Content: windChatSystemPrompt(s, wind, last, previous)}}, turns...)
}
