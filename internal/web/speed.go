package web

import (
	"sync"
	"time"

	"gregal/internal/llm"
)

// Velocitat de cada crida al model per a la UI: pp (processament del
// prompt, tokens/s fins al primer tros de resposta) i tg (generació,
// tokens/s des d'aleshores fins al final). Amb models locals és el que diu
// si el que va lent és llegir el context o escriure; la UI en fa la
// mitjana del torn.
//
// Els tokens surten de l'usage del proveïdor (la diferència del comptador
// del torn abans i després de la crida). Si el proveïdor no en dona, la
// generació s'estima pels caràcters rebuts (≈4 per token) i el prompt per
// l'historial, i l'event ho marca amb estimated.

type speedProbe struct {
	mu        sync.Mutex
	meter     *llm.UsageMeter
	t0, first time.Time
	chars     int
	before    llm.Usage
	withUsage int
}

func newSpeedProbe(meter *llm.UsageMeter) *speedProbe {
	p := &speedProbe{meter: meter, t0: time.Now()}
	if meter != nil {
		p.before, _, p.withUsage = meter.Total()
	}
	return p
}

// delta és un llm.StreamProgress: apunta el primer tros i compta caràcters.
func (p *speedProbe) delta(kind string, n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if kind == llm.ProgressFirst {
		if p.first.IsZero() {
			p.first = time.Now()
		}
		return
	}
	p.chars += n
}

// result torna l'event "speed" de la crida, o nil si no hi ha res a dir
// (cap resposta, o una crida massa curta per mesurar).
func (p *speedProbe) result(end time.Time, estPrompt int) map[string]any {
	p.mu.Lock()
	first, chars := p.first, p.chars
	p.mu.Unlock()
	if first.IsZero() {
		return nil
	}
	prompt, cached, completion, estimated := 0, 0, 0, false
	if p.meter != nil {
		after, _, with := p.meter.Total()
		if with > p.withUsage {
			prompt = after.PromptTokens - p.before.PromptTokens
			cached = after.PromptTokensDetails.CachedTokens - p.before.PromptTokensDetails.CachedTokens
			completion = after.CompletionTokens - p.before.CompletionTokens
		}
	}
	if completion <= 0 {
		completion, estimated = (chars+3)/4, true
	}
	if prompt <= 0 {
		prompt, estimated = estPrompt, true
	}
	pp, tg := speedOf(prompt, cached, completion, first.Sub(p.t0), end.Sub(first))
	if pp == 0 && tg == 0 {
		return nil
	}
	return map[string]any{
		"pp": pp, "tg": tg,
		"prompt": prompt, "cached": cached, "completion": completion,
		"ttft_ms": first.Sub(p.t0).Milliseconds(), "gen_ms": end.Sub(first).Milliseconds(),
		"estimated": estimated,
	}
}

// speedOf calcula pp i tg en tokens/s (0 si no es pot mesurar). El pp
// compta només els tokens que el model ha hagut de processar: els que el
// proveïdor diu que venien de la cache no hi entren. Amb menys de
// minPPTokens per processar el temps fins al primer token és latència (xarxa,
// cua), no processament: vist en viu, 14 tokens nous en 2 s sortien com a
// «pp 80 t/s» amb el prompt a la cache.
const minPPTokens = 512

func speedOf(prompt, cached, completion int, ttft, gen time.Duration) (pp, tg float64) {
	if processed := prompt - cached; processed >= minPPTokens && ttft >= 50*time.Millisecond {
		pp = round1(float64(processed) / ttft.Seconds())
	}
	if completion > 1 && gen >= 50*time.Millisecond {
		tg = round1(float64(completion) / gen.Seconds())
	}
	return pp, tg
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
