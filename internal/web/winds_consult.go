package web

import (
	"context"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Quan un vent en pregunta a un altre (eina wind, ask o all), el xat diu a qui
// pregunta mentre ho fa i, en acabar el torn, el fil del vent que pregunta
// guarda què li ha respost cada vent (missatges «consult», plegats a l'app).
// Les respostes no tornen al model: ja les va tenir com a resultat de l'eina.

const consultKind = "consult"

type windAskingKey struct{}

// withWindAsking: el bucle hi posa com anunciar els vents consultats; només
// runWindTool els sap, un cop resolts contra el magatzem de vents.
func withWindAsking(ctx context.Context, announce func(ids []string)) context.Context {
	return context.WithValue(ctx, windAskingKey{}, announce)
}

func announceWindAsking(ctx context.Context, ws []companion.Wind) {
	announce, ok := ctx.Value(windAskingKey{}).(func([]string))
	if !ok {
		return
	}
	ids := make([]string, 0, len(ws))
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	announce(ids)
}

type windConsultsKey struct{}

// windConsults recull, durant un torn, el que han respost els vents consultats.
type windConsults struct {
	mu   sync.Mutex
	msgs []llm.Message
}

func withWindConsults(ctx context.Context, c *windConsults) context.Context {
	return context.WithValue(ctx, windConsultsKey{}, c)
}

func noteWindConsult(ctx context.Context, w companion.Wind, text string) {
	c, ok := ctx.Value(windConsultsKey{}).(*windConsults)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, llm.Message{Role: "assistant", Kind: consultKind, Content: text,
		SpeakerID: w.ID, SpeakerName: w.Name, Ts: time.Now().Unix()})
}

func (c *windConsults) taken() []llm.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]llm.Message(nil), c.msgs...)
}
