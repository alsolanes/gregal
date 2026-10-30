package web

import (
	"context"
	"net/http"
)

// L'Arena (arena*.go) com a plugin: competició de prompts entre .md, amb
// retransmissió i veus. En arrencar reprèn les competicions que una
// aturada va deixar a mitges.
func init() {
	registerPlugin(&Plugin{
		ID: "arena", Order: 30, DefaultOn: true, Icon: "🏆", View: "arena",
		Name: "Arena", NameCA: "Arena",
		Description:   "Prompt competition: each player writes a .md strategy and the model plays it against the others, with a live broadcast.",
		DescriptionCA: "Competició de prompts: cada jugador escriu una estratègia .md i el model la juga contra les altres, amb retransmissió en directe.",
		Routes: func(h *Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/arena", h.global((*Server).handleArena))
			handle("/api/arena/match", h.global((*Server).handleArenaMatch))
			handle("/api/arena/competition", h.global((*Server).handleArenaComp))
			handle("/api/arena/stream", h.global((*Server).handleArenaStream))
			handle("/api/arena/starter", h.global((*Server).handleArenaStarter))
			handle("/api/arena/voice", h.global((*Server).handleArenaVoice))
		},
		Start: func(ctx context.Context, s *Server) { s.arenaResume(ctx) },
		Stop:  func(*Server) { arenaPauseAll() },
	})
}

// arenaPauseAll atura tot el que l'Arena té en marxa quan s'apaga: les
// competicions queden «running» al disc (en tornar-la a encendre,
// arenaResume les continua pel partit que tocava) i els partits en joc
// s'aturen.
func arenaPauseAll() {
	arenaCompMu.Lock()
	for id, cancel := range arenaCompCancel {
		cancel()
		delete(arenaCompCancel, id)
	}
	arenaCompMu.Unlock()
	arenaRunsMu.Lock()
	for _, r := range arenaRuns {
		if r != nil && r.cancel != nil {
			r.cancel()
		}
	}
	arenaRunsMu.Unlock()
}
