package agent

import (
	"testing"
)

func TestCostMath(t *testing.T) {
	SetPrices(map[string]Price{"kimi-k2.7": {In: 0.6, Out: 2.5}})
	defer SetPrices(nil)
	usd, ok := CostUSD("cloud", "kimi-k2.7", 1_000_000, 1_000_000)
	if !ok || usd != 3.1 {
		t.Fatalf("usd=%v ok=%v", usd, ok)
	}
	// Clau provider/model té prioritat.
	SetPrices(map[string]Price{"cloud/kimi-k2.7": {In: 1, Out: 1}, "kimi-k2.7": {In: 9, Out: 9}})
	usd, _ = CostUSD("cloud", "kimi-k2.7", 1_000_000, 0)
	if usd != 1 {
		t.Fatalf("prioritat provider/model: %v", usd)
	}
}

func TestCostDesconegutNoInvents(t *testing.T) {
	SetPrices(map[string]Price{"kimi-k2.7": {In: 0.6, Out: 2.5}})
	defer SetPrices(nil)
	if _, ok := CostUSD("local-swap", "Ornith-1.5-35B", 999999, 999999); ok {
		t.Fatal("sense preu no hi ha cost")
	}
	SetPrices(nil)
	if _, ok := CostUSD("cloud", "kimi-k2.7", 1, 1); ok {
		t.Fatal("taula buida = sense cost")
	}
}

func TestFmtCost(t *testing.T) {
	for in, want := range map[float64]string{
		0.00042: "$0.0004", 0.0234: "$0.023", 1.2: "$1.200", 12.345: "$12.35",
	} {
		if got := FmtCost(in); got != want {
			t.Fatalf("FmtCost(%v)=%q, volia %q", in, got, want)
		}
	}
}
