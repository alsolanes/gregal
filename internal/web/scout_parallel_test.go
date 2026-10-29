package web

import (
	"testing"

	"gregal/internal/config"
)

// La decisió de córrer el scout en paral·lel depèn del proveïdor, no del gust:
// amb una sola ranura (model local) el paral·lel empitjora el torn, mesurat
// (58 s → 2m18s a la mateixa pregunta).
func TestScoutEnParallelNomesAmbRancures(t *testing.T) {
	casos := []struct {
		nom  string
		mode string
		cfg  *config.Config
		vol  bool
	}{
		{
			nom:  "mode inspect sempre espera: el brief és la resposta",
			mode: "inspect",
			cfg:  &config.Config{Providers: map[string]config.Provider{"p": {BaseURL: "https://api.exemple.com/v1"}}},
			vol:  false,
		},
		{
			nom:  "local (loopback) = una ranura = en sèrie",
			mode: "code",
			cfg: &config.Config{
				Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1:8731/v1"}},
				Roles:     map[string]config.Role{"code": {Provider: "p", Model: "m"}},
			},
			vol: false,
		},
		{
			nom:  "núvol = pot servir dues crides = en paral·lel",
			mode: "code",
			cfg: &config.Config{
				Providers: map[string]config.Provider{"p": {BaseURL: "https://api.exemple.com/v1"}},
				Roles:     map[string]config.Role{"code": {Provider: "p", Model: "m"}},
			},
			vol: true,
		},
		{
			nom:  "parallel_subagents forçat a 4 guanya la detecció de local",
			mode: "code",
			cfg: &config.Config{
				Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1:8731/v1"}},
				Roles:     map[string]config.Role{"code": {Provider: "p", Model: "m"}},
				Agent:     config.AgentCfg{ParallelSubagents: 4},
			},
			vol: true,
		},
		{
			nom:  "scout_parallel: off ho apaga del tot",
			mode: "code",
			cfg: &config.Config{
				Providers: map[string]config.Provider{"p": {BaseURL: "https://api.exemple.com/v1"}},
				Roles:     map[string]config.Role{"code": {Provider: "p", Model: "m"}},
				Agent:     config.AgentCfg{ScoutParallel: "off"},
			},
			vol: false,
		},
	}
	for _, c := range casos {
		s := &Server{cfg: c.cfg}
		if got := s.scoutEnParallel(c.mode); got != c.vol {
			t.Fatalf("%s: scoutEnParallel(%q) = %v, volia %v", c.nom, c.mode, got, c.vol)
		}
	}
}
