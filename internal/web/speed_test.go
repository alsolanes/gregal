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

func TestSpeedProbeUsesProviderUsageOrEstimates(t *testing.T) {
	meter := &llm.UsageMeter{}
	p := newSpeedProbe(meter)
	p.t0 = time.Now().Add(-3 * time.Second)
	p.delta(llm.ProgressFirst, 0)
	p.first = p.t0.Add(time.Second)
	p.delta(llm.ProgressText, 400)
	meter.Add(llm.Usage{PromptTokens: 2000, CompletionTokens: 100})
	ev := p.result(p.first.Add(2*time.Second), 999)
	if ev["pp"] != 2000.0 || ev["tg"] != 50.0 || ev["estimated"] != false {
		t.Fatalf("with usage: %v", ev)
	}

	q := newSpeedProbe(nil)
	q.t0 = time.Now().Add(-3 * time.Second)
	q.delta(llm.ProgressFirst, 0)
	q.first = q.t0.Add(time.Second)
	q.delta(llm.ProgressText, 400)
	ev = q.result(q.first.Add(2*time.Second), 1500)
	if ev["completion"] != 100 || ev["prompt"] != 1500 || ev["estimated"] != true {
		t.Fatalf("estimated: %v", ev)
	}
	if newSpeedProbe(nil).result(time.Now(), 10) != nil {
		t.Fatal("a call with no response must not report a speed")
	}
}
