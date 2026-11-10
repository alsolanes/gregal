package cs2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestLiveFixture escriu internal/web/testdata/cs2-live-fixture.json: el meta
// i rondes preparades com les que rep el navegador, per a les proves de la
// simulació (arena-cs2-live.test.cjs) i per calibrar els bots. Només amb
// CS2_LIVE_FIXTURE=<fitxer>.
func TestLiveFixture(t *testing.T) {
	path := os.Getenv("CS2_LIVE_FIXTURE")
	if path == "" {
		t.Skip("set CS2_LIVE_FIXTURE to regenerate")
	}
	out := map[string]any{}
	l := NewLive(9, "en", [2]string{"A", "B"}, 6, 0, 2)
	out["meta"] = l.Match().Meta()
	var setups []any
	for _, a := range []string{`{"strategy":"execute_a","setup":"standard_2_1_2"}`, `{"strategy":"rush_b","setup":"stack_a"}`, `{"strategy":"default_mid","setup":"aggressive_mid"}`, `{"strategy":"fake_a_go_b","setup":"standard_2_1_2","utility":"full"}`} {
		l.Prompts()
		s, _ := l.Start([2]json.RawMessage{json.RawMessage(a), json.RawMessage(a)})
		setups = append(setups, s)
		l.Finish(LiveResult{})
	}
	out["setups"] = setups
	// Totes les estratègies contra tots els muntatges, amb compra completa.
	var full []any
	for _, st := range stratVals {
		for _, se := range setupVals {
			l := NewLive(int64(len(full)+1), "en", [2]string{"A", "B"}, 12, 0, 2)
			l.Prompts()
			for tm := 0; tm < 2; tm++ {
				for _, p := range l.m.players[tm] {
					p.Money = 16000
				}
			}
			a := json.RawMessage(fmt.Sprintf(`{"buy":"full","strategy":%q,"utility":"full","pace":"default","setup":%q}`, st, se))
			s, _ := l.Start([2]json.RawMessage{a, a})
			full = append(full, s)
		}
	}
	out["full"] = full
	raw, _ := json.Marshal(out)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
