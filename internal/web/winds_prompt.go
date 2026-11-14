package web

import (
	"context"
	"strings"

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
func windChatTurns(previous []llm.Message) []llm.Message {
	var picked []llm.Message
	total := 0
	for i := len(previous) - 1; i >= 0 && len(picked) < windTurnsMaxMsgs; i-- {
		m := previous[i]
		if m.Role != "user" && m.Role != "assistant" || m.Kind == memoryKind {
			continue
		}
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		if r := []rune(text); len(r) > windTurnMaxChars {
			text = string(r[:windTurnMaxChars]) + " […]"
		}
		if total+len(text) > windTurnsMaxChars && len(picked) > 0 {
			break
		}
		total += len(text)
		picked = append(picked, llm.Message{Role: m.Role, Content: text})
	}
	// De més antic a més nou.
	for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
		picked[i], picked[j] = picked[j], picked[i]
	}
	var out []llm.Message
	for _, m := range picked {
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Content += "\n\n" + m.Content
			continue
		}
		out = append(out, m)
	}
	if len(out) > 0 && out[0].Role == "assistant" {
		out = append([]llm.Message{{Role: "user", Content: "(earlier conversation)"}}, out...)
	}
	return out
}

// windTurnMessages munta system + torns + missatge nou. Si l'últim torn desat
// ja és de l'usuari (un missatge sense resposta), s'hi enganxa el nou.
func windTurnMessages(system string, previous []llm.Message, msg string) []llm.Message {
	msgs := []llm.Message{{Role: "system", Content: system}}
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
	return append([]llm.Message{{Role: "system", Content: windChatSystemPrompt(s, wind, last)}}, turns...)
}
