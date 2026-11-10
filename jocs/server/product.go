package server

import (
	"context"
	"net/http"

	"gregal/internal/web"
	jocsweb "gregal/jocs/web"
)

// Gregal Jocs com a producte del servidor de Gregal (internal/web/product.go):
// la pàgina /jocs/, els seus fitxers web a /app/, l'espectador de l'Arena i
// dos plugins, la lliga dels vents (Partits) i l'Arena. Només hi és en el
// binari que importa aquest paquet (jocs/cmd/gregal-jocs); el de Gregal no.
func init() {
	web.RegisterProduct(Product())
}

// Product és la descripció de Gregal Jocs per al registre de productes.
func Product() *web.Product {
	return &web.Product{
		ID: "jocs", Path: "/jocs",
		Title: `<title data-i18n="ui.titleJocs">Gregal Jocs</title>`,
		Slots: map[string]string{
			"css":     `<link rel="stylesheet" href="/app/jocs-site.css">`,
			"modules": `<script type="module" src="/app/football.js"></script>` + "\n" + `<script type="module" src="/app/arena.js"></script>`,
			"end":     `<script type="module" src="/app/jocs-site.js"></script>`,
		},
		Assets:  jocsweb.Assets,
		Plugins: []*web.Plugin{gamesPlugin(), arenaPlugin()},
		Guest:   func(s *web.Server, r *http.Request) bool { return New(s).arenaSpectatorOK(r) },
	}
}

// gamesPlugin: els partits de futbol americà de la lliga (football.go, amb
// els equips de teams.go). No té bucle de fons: un partit només es juga
// quan algú el demana.
func gamesPlugin() *web.Plugin {
	return &web.Plugin{
		ID: "games", Order: 20, DefaultOn: true, Icon: "🏈", View: "partits",
		Name: "Games", NameCA: "Partits",
		Description:   "The winds' American football league: teams led by the league's members, games played and narrated by the model.",
		DescriptionCA: "La lliga de futbol americà dels vents: equips dirigits pels membres de la lliga, partits jugats i narrats pel model.",
		Routes: func(h *web.Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/football", global(h, (*Server).handleFootball))
			handle("/api/football/match", global(h, (*Server).handleFootballMatch))
		},
	}
}

// arenaPlugin: l'Arena (arena*.go), competició de prompts entre .md, amb
// retransmissió i veus. En arrencar reprèn les competicions que una
// aturada va deixar a mitges.
func arenaPlugin() *web.Plugin {
	return &web.Plugin{
		ID: "arena", Order: 30, DefaultOn: true, Icon: "🏆", View: "arena",
		Name: "Arena", NameCA: "Arena",
		Description:   "Prompt competition: each player writes a .md strategy and the model plays it against the others, with a live broadcast.",
		DescriptionCA: "Competició de prompts: cada jugador escriu una estratègia .md i el model la juga contra les altres, amb retransmissió en directe.",
		Routes: func(h *web.Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/arena", global(h, (*Server).handleArena))
			handle("/api/arena/match", global(h, (*Server).handleArenaMatch))
			handle("/api/arena/competition", global(h, (*Server).handleArenaComp))
			handle("/api/arena/stream", global(h, (*Server).handleArenaStream))
			handle("/api/arena/starter", global(h, (*Server).handleArenaStarter))
			handle("/api/arena/voice", global(h, (*Server).handleArenaVoice))
			handle("/api/arena/live", global(h, (*Server).handleArenaLive))
		},
		Start: func(ctx context.Context, s *web.Server) { New(s).arenaResume(ctx) },
		Stop:  func(*web.Server) { arenaPauseAll() },
	}
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
