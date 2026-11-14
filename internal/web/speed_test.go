package web

import (
	"testing"
	"time"

	"gregal/internal/llm"
)

func TestSpeedOfSplitsPromptAndGeneration(t *testing.T) {
	// 32k de prompt en 22,5 s (prefill del qwen local) i 380 tokens en 10 s.
	pp, tg := speedOf(32000, 0, 380, 22500*time.Millisecond, 10*time.Second)
	if pp != 1422.2 || tg != 38 {
		t.Fatalf("pp=%v tg=%v, want 1422.2 and 38", pp, tg)
	}
	// La part que ve de la cache no compta com a processada.
	if pp, _ := speedOf(41000, 39000, 10, 2*time.Second, time.Second); pp != 1000 {
		t.Fatalf("cached tokens counted: pp=%v", pp)
	}
	// Quasi tot a la cache: el temps és latència, no processament.
	if pp, _ := speedOf(10782, 10624, 46, 1968*time.Millisecond, 356*time.Millisecond); pp != 0 {
		t.Fatalf("latency reported as pp=%v", pp)
	}
	// Massa curt per mesurar: zero, no un número absurd.
	if pp, tg := speedOf(100, 0, 1, 10*time.Millisecond, 10*time.Millisecond); pp != 0 || tg != 0 {
		t.Fatalf("pp=%v tg=%v, want 0", pp, tg)
	}
}

// streamed simula una resposta: primer token a t0+ttft, n trossos fins a t0+ttft+gen.
func streamed(p *speedProbe, ttft, gen time.Duration, n, chars int) {
	p.t0 = time.Now().Add(-time.Hour)
	p.first, p.last = p.t0.Add(ttft), p.t0.Add(ttft+gen)
	p.deltas, p.chars = n, chars
}

func TestSpeedProbeUsesProviderUsageOrEstimates(t *testing.T) {
	meter := &llm.UsageMeter{}
	p := newSpeedProbe(meter)
	streamed(p, time.Second, 2*time.Second, 40, 400)
	meter.Add(llm.Usage{PromptTokens: 2000, CompletionTokens: 100})
	ev := p.result(p.last, 999)
	if ev["pp"] != 2000.0 || ev["tg"] != 50.0 || ev["estimated"] != false {
		t.Fatalf("with usage: %v", ev)
	}

	q := newSpeedProbe(nil)
	streamed(q, time.Second, 2*time.Second, 40, 400)
	ev = q.result(q.last, 1500)
	if ev["completion"] != 100 || ev["prompt"] != 1500 || ev["estimated"] != true {
		t.Fatalf("estimated: %v", ev)
	}
	if newSpeedProbe(nil).result(time.Now(), 10) != nil {
		t.Fatal("a call with no response must not report a speed")
	}
}

// Molts servidors envien de seguida un tros buit (només el rol): no és el
// primer token. Abans el pp sortia altíssim perquè el rellotge parava aquí.
func TestSpeedProbeIgnoresTheEmptyFirstChunk(t *testing.T) {
	meter := &llm.UsageMeter{}
	p := newSpeedProbe(meter)
	p.delta(llm.ProgressFirst, 0)
	p.delta(llm.ProgressText, 0)
	p.mu.Lock()
	empty := p.first.IsZero()
	p.mu.Unlock()
	if !empty {
		t.Fatal("an empty first chunk started the clock")
	}
	p.delta(llm.ProgressReasoning, 12)
	if p.first.IsZero() || p.deltas != 1 {
		t.Fatalf("the first real token did not start the clock: %+v", p)
	}
}

// Un proxy que reté la resposta i la deixa anar de cop: la tg no es pot
// mesurar i no s'ensenya cap número impossible.
func TestSpeedProbeSkipsBurstGeneration(t *testing.T) {
	meter := &llm.UsageMeter{}
	p := newSpeedProbe(meter)
	streamed(p, 20*time.Second, 30*time.Millisecond, 300, 6000)
	meter.Add(llm.Usage{PromptTokens: 20000, CompletionTokens: 1500})
	ev := p.result(p.last, 0)
	if ev == nil || ev["tg"] != 0.0 || ev["pp"] != 1000.0 {
		t.Fatalf("burst: %v", ev)
	}
}

// Si el servidor mesura ell mateix (timings de llama.cpp, halogen), mana la seva.
func TestSpeedProbePrefersServerTimings(t *testing.T) {
	meter := &llm.UsageMeter{}
	meter.AddTimings(&llm.LocalTimings{PromptN: 9000, PromptPerSecond: 1, PredictedPerSecond: 1}) // d'una crida anterior
	p := newSpeedProbe(meter)
	streamed(p, time.Second, 30*time.Millisecond, 3, 400)
	meter.Add(llm.Usage{PromptTokens: 9000, CompletionTokens: 900})
	meter.AddTimings(&llm.LocalTimings{PromptN: 8000, CacheN: 1000, PromptPerSecond: 812.44, PredictedN: 900, PredictedPerSecond: 41.27})
	ev := p.result(p.last, 0)
	if ev["pp"] != 812.4 || ev["tg"] != 41.3 || ev["source"] != "server" || ev["estimated"] != false || ev["cached"] != 1000 {
		t.Fatalf("server timings: %v", ev)
	}
	// Una crida sense timings no agafa els d'una altra.
	q := newSpeedProbe(meter)
	streamed(q, time.Second, 2*time.Second, 40, 400)
	meter.Add(llm.Usage{PromptTokens: 2000, CompletionTokens: 100})
	if ev := q.result(q.last, 0); ev["source"] == "server" {
		t.Fatalf("reused old timings: %v", ev)
	}
}
