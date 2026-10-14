package llm

import (
	"context"
	"unicode/utf8"
)

// StreamProgress rep un senyal per cada delta que arriba d'una resposta en
// streaming, tant si el text surt en directe (onToken/onReason) com si es
// reté fins al final (mode recuperació, WithRecoverTruncation). Serveix per
// dir "el model és viu i està escrivint" sense ensenyar res que encara es
// pugui descartar.
//
// kind és ProgressFirst (una vegada per resposta HTTP, amb n=0, quan arriba
// el primer tros de dades), ProgressReasoning, ProgressText o ProgressTool
// (n = caràcters del delta: raonament, text o arguments d'una crida d'eina).
// Es crida al mateix goroutine que llegeix el stream: ha de ser ràpida i no
// bloquejar.
//
// Vist en viu el 2026-10-13: un torn autònom de la web va passar 2m47s
// sense cap senyal mentre el model raonava (el conductor no passava
// onReason i el mode recuperació no deixa sortir res fins al final), i
// l'usuari no podia saber si l'agent s'havia penjat.
type StreamProgress func(kind string, n int)

// Tipus de delta que rep StreamProgress.
const (
	ProgressFirst     = "first"
	ProgressReasoning = "reasoning"
	ProgressText      = "text"
	ProgressTool      = "tool"
)

type streamProgressKey struct{}

// WithStreamProgress adjunta al context l'observador de deltes. Amb fn nil
// el context no canvia (cap cost ni cap comportament nou).
// Si el context ja en porta un, s'encadenen: el límit d'inactivitat del
// pas (agent.ModelContext) i el progrés que pinta la UI han de veure tots
// dos els mateixos deltes.
func WithStreamProgress(ctx context.Context, fn StreamProgress) context.Context {
	if fn == nil {
		return ctx
	}
	if prev := StreamProgressFrom(ctx); prev != nil {
		next := fn
		fn = func(kind string, n int) { prev(kind, n); next(kind, n) }
	}
	return context.WithValue(ctx, streamProgressKey{}, fn)
}

// StreamProgressFrom torna l'observador del context (nil si no n'hi ha).
func StreamProgressFrom(ctx context.Context) StreamProgress {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(streamProgressKey{}).(StreamProgress)
	return fn
}

// progresResposta és l'estat d'una sola resposta HTTP: recorda si ja s'ha
// avisat del primer tros. Un punter nil (sense observador) no fa res, de
// manera que els parsers el poden cridar sense comprovar-ho.
type progresResposta struct {
	fn   StreamProgress
	vist bool
}

func nouProgres(ctx context.Context) *progresResposta {
	fn := StreamProgressFrom(ctx)
	if fn == nil {
		return nil
	}
	return &progresResposta{fn: fn}
}

// tros marca que ha arribat un tros de dades (avisa la primera vegada).
func (p *progresResposta) tros() {
	if p == nil || p.vist {
		return
	}
	p.vist = true
	p.fn(ProgressFirst, 0)
}

// delta avisa d'un delta no buit del tipus donat, en caràcters.
func (p *progresResposta) delta(kind, s string) {
	if p == nil || s == "" {
		return
	}
	p.fn(kind, utf8.RuneCountInString(s))
}
