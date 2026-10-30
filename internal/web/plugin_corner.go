package web

import (
	"context"
	"net/http"
)

// El Racó dels vents (corner.go) com a plugin: la pàgina que fan ells sols,
// amb el seu rellotge de reunions.
func init() {
	registerPlugin(&Plugin{
		ID: "corner", Order: 10, DefaultOn: true, Icon: "✦", View: "raco",
		Name: "Corner", NameCA: "Racó",
		Description:   "A page the winds build and review on their own, with a journal and versions you can revert.",
		DescriptionCA: "Una pàgina que els vents construeixen i revisen sols, amb diari i versions que pots desfer.",
		Routes: func(h *Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/corner", h.global((*Server).handleCorner))
			handle("/api/corner/page", h.global((*Server).handleCornerPage))
			handle("/api/corner/ask", h.global((*Server).handleCornerAsk))
		},
		Start: func(ctx context.Context, s *Server) { s.cornerLoop(ctx) },
	})
}
