package llm

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// UsageMeter suma el consum real (usage) de totes les crides fetes amb un
// mateix context. Serveix per saber què ha costat de debò una execució
// sencera: cada pas torna a enviar tot el prompt, i l'estimació sobre
// l'historial final no ho veu (un run de cinc passos donava 270 tokens de
// pujada quan en eren milers).
//
// Viatja amb el context (WithUsage) com els avisos de reintent: cap firma
// de Chat* canvia i les crides internes (compactació, síntesi, revisió)
// també hi sumen perquè reben el mateix context. És segur entre goroutines.
type UsageMeter struct {
	mu        sync.Mutex
	total     Usage
	calls     int
	withUsage int
	// parent és el comptador que ja hi havia al context quan s'ha penjat
	// aquest: un run dins d'un altre (verificació, flows) suma a tots dos.
	parent *UsageMeter
}

// Add suma el consum d'una crida. Una crida sense usage (el proveïdor no
// el declara) compta com a crida però no suma tokens: així qui llegeix pot
// distingir «zero tokens» de «ningú ho ha dit».
func (m *UsageMeter) Add(u Usage) {
	if m == nil {
		return
	}
	defer m.parent.Add(u)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if u.Buida() {
		return
	}
	m.withUsage++
	m.total.PromptTokens += u.PromptTokens
	m.total.CompletionTokens += u.CompletionTokens
	tot := u.TotalTokens
	if tot == 0 {
		tot = u.PromptTokens + u.CompletionTokens
	}
	m.total.TotalTokens += tot
	m.total.PromptTokensDetails.CachedTokens += u.PromptTokensDetails.CachedTokens
}

// Total torna la suma, quantes crides hi ha hagut i quantes han declarat
// consum. Si withUsage és 0, el proveïdor no n'ha dit res i cal estimar.
func (m *UsageMeter) Total() (u Usage, calls, withUsage int) {
	if m == nil {
		return Usage{}, 0, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.total, m.calls, m.withUsage
}

type usageKey struct{}

// WithUsage adjunta un comptador de consum al context: totes les crides
// Chat* que el rebin hi sumaran el seu usage. Si el context ja en portava
// un, el nou hi queda encadenat i el consum arriba als dos.
func WithUsage(ctx context.Context, m *UsageMeter) context.Context {
	if m == nil {
		return ctx
	}
	if prev := UsageFrom(ctx); prev != nil && prev != m && m.parent == nil {
		m.parent = prev
	}
	return context.WithValue(ctx, usageKey{}, m)
}

// UsageFrom torna el comptador del context (nil si no n'hi ha).
func UsageFrom(ctx context.Context) *UsageMeter {
	if m, ok := ctx.Value(usageKey{}).(*UsageMeter); ok {
		return m
	}
	return nil
}

// usageCall recull el consum d'una sola crida HTTP. En streaming, alguns
// proveïdors repeteixen usage a cada event amb el total acumulat: per això
// es guarda l'últim que arriba i no se sumen tots. done el passa al
// comptador del context una sola vegada.
type usageCall struct {
	c    *Client
	m    *UsageMeter
	last Usage
}

func (c *Client) usageCall(ctx context.Context) *usageCall {
	return &usageCall{c: c, m: UsageFrom(ctx)}
}

func (r *usageCall) see(u Usage) {
	if u.Buida() {
		return
	}
	r.last = u
	r.c.setUsage(u)
}

func (r *usageCall) done() {
	r.m.Add(r.last)
}

// streamOptions demana l'usage a l'últim event d'un stream (OpenAI i la
// majoria de compatibles). Sense això, en streaming no arriba cap consum.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// postStream és postRetry per a les crides amb stream:true: hi afegeix
// stream_options.include_usage. Hi ha proxies i servidors estrictes que
// rebutgen camps desconeguts amb 400/422: en aquest cas es torna a provar
// una vegada sense el camp i, si així va bé, aquell endpoint ja no el rep
// mai més (en aquest procés). No saber el consum és molt millor que trencar
// la crida.
func (c *Client) postStream(ctx context.Context, baseURL, apiKey string, req chatRequest) (*http.Response, error) {
	// La negociació (treure el camp si el proveïdor el rebutja i
	// recordar-ho) és la mateixa per a tots els camps opcionals: compat.go.
	req.StreamOptions = &streamOptions{IncludeUsage: true}
	return c.postCompat(ctx, baseURL, apiKey, req)
}

// rebutjaCampDesconegut diu si un 400 sembla el d'un camp que el servidor no
// accepta. Els missatges varien molt (OpenAI, vLLM, pydantic, proxies), així
// que es busca el nom del camp o les fórmules habituals.
func rebutjaCampDesconegut(body string) bool {
	b := strings.ToLower(body)
	for _, s := range []string{
		"stream_options", "include_usage", "unrecognized", "unknown field",
		"unknown parameter", "extra inputs", "extra_forbidden", "not permitted",
		"additional properties", "unsupported parameter",
	} {
		if strings.Contains(b, s) {
			return true
		}
	}
	return false
}
