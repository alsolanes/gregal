package web

import (
	"embed"
	"encoding/json"
	"strings"
)

// Partits de demostració: tres partits de debò (el model jugant un .md expert
// contra un de principiant, amb comentaris, anàlisi i informes) desats dins
// del binari, per ensenyar com és un partit sense esperar ni gastar crides.
// S'hi accedeix com a qualsevol partit, amb l'id «demo-nfl», «demo-cs2» o
// «demo-lol».
//
//go:embed arena_demo/*.json arena_demo/*.sit
var arenaDemoFS embed.FS

func arenaDemoGame(id string) string {
	g := strings.TrimPrefix(id, "demo-")
	if g == id || strings.ContainsAny(g, "./\\") {
		return ""
	}
	return g
}

func arenaDemo(id string) *arenaMatch {
	g := arenaDemoGame(id)
	if g == "" {
		return nil
	}
	raw, err := arenaDemoFS.ReadFile("arena_demo/" + g + ".json")
	if err != nil {
		return nil
	}
	var m arenaMatch
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return &m
}

func arenaDemoSits(id string) []arenaSituation {
	g := arenaDemoGame(id)
	if g == "" {
		return nil
	}
	raw, err := arenaDemoFS.ReadFile("arena_demo/" + g + ".sit")
	if err != nil {
		return nil
	}
	var out []arenaSituation
	_ = json.Unmarshal(raw, &out)
	return out
}

// arenaDemoVoice: la veu de la demo es fa en aquest ordinador, amb les veus
// que hi hagi (la gravació no porta àudio). Sense veus, subtítols. La primera
// vegada les frases es van generant; després surten de la memòria cau.
func (s *Server) arenaDemoVoice(m *arenaMatch) {
	ready := s.voiceReady("en")
	say := func(text, who string) string {
		if !ready || text == "" {
			return ""
		}
		return s.voiceLater(text, who, "en")
	}
	m.PreviewAudio = say(m.Preview, "analyst")
	for i := range m.Lines {
		m.Lines[i].Audio = say(m.Lines[i].Text, "caster")
	}
	if m.Analysis != nil {
		m.Analysis.Audio = say(m.Analysis.Headline+". "+m.Analysis.Text, "analyst")
	}
}
