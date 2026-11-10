package server

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// arenaEventCfg: el nom i la data de la competició tal com surten a la web de
// Gregal Jocs. L'organitzador (admin) ho canvia; es desa a settings.json com
// {"event": {...}}, de manera que el nom de l'empresa no és mai al codi.
type arenaEventCfg struct {
	Name    string `json:"name"`
	Tagline string `json:"tagline"`
	Date    int64  `json:"date"` // unix; 0 = sense data
	Place   string `json:"place"`
}

const (
	arenaEventDefaultName    = "Gregal Prompt Cup"
	arenaEventDefaultTagline = "Write the prompt. The model plays. The best instructions win."
)

func (s *Server) arenaEvent() arenaEventCfg {
	var st struct {
		Event arenaEventCfg `json:"event"`
	}
	arenaRead(filepath.Join(s.arenaDir(), "settings.json"), &st)
	c := st.Event
	if strings.TrimSpace(c.Name) == "" {
		c.Name = arenaEventDefaultName
	}
	if strings.TrimSpace(c.Tagline) == "" {
		c.Tagline = arenaEventDefaultTagline
	}
	return c
}

func (s *Server) saveArenaEvent(c arenaEventCfg) arenaEventCfg {
	clip := func(v string, n int) string {
		v = strings.TrimSpace(v)
		if r := []rune(v); len(r) > n {
			v = string(r[:n])
		}
		return v
	}
	c.Name, c.Tagline, c.Place = clip(c.Name, 60), clip(c.Tagline, 140), clip(c.Place, 80)
	if c.Date < 0 {
		c.Date = 0
	}
	arenaMu.Lock()
	path := filepath.Join(s.arenaDir(), "settings.json")
	raw := map[string]json.RawMessage{}
	arenaRead(path, &raw)
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(c)
	raw["event"] = b
	arenaWrite(path, raw)
	arenaMu.Unlock()
	return s.arenaEvent()
}

// arenaTotals: les xifres de la portada (tots els partits, no només els
// recents que veu cadascú).
func arenaTotals(sums []arenaSum) map[string]int {
	players := map[string]bool{}
	t := map[string]int{}
	for i := range sums {
		m := &sums[i].M
		t["matches"]++
		if m.Kind == "official" {
			t["official"]++
		}
		if m.Stats != nil {
			t["decisions"] += m.Stats.Decisions
		}
		for _, o := range m.Owners {
			if o != "" && !strings.HasPrefix(o, "bot:") {
				players[o] = true
			}
		}
	}
	t["players"] = len(players)
	return t
}
