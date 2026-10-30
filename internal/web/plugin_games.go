package web

import "net/http"

// Els partits de futbol americà entre els vents (football.go) com a plugin.
// No té bucle de fons: un partit només es juga quan algú el demana.
func init() {
	registerPlugin(&Plugin{
		ID: "games", Order: 20, DefaultOn: true, Icon: "🏈", View: "partits",
		Name: "Games", NameCA: "Partits",
		Description:   "The winds' American football league: teams made of your winds, games played and narrated by the model.",
		DescriptionCA: "La lliga de futbol americà dels vents: equips fets amb els teus vents, partits jugats i narrats pel model.",
		Routes: func(h *Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/football", h.global((*Server).handleFootball))
			handle("/api/football/match", h.global((*Server).handleFootballMatch))
		},
	})
}
