package web

import (
	"encoding/base64"
	"net/http"
	"time"

	"gregal/internal/tools"
)

// liveShot és variable perquè els tests no hagin d'engegar cap navegador.
var liveShot = tools.LiveShot

// handleOrdinadorLive és GET /api/ordinador/live: en mode local (l'ordinador
// de Gregal és el navegador d'aquest servidor, sense VM ni noVNC), una captura
// actual de la pestanya on treballa, amb l'adreça i el títol. El mòbil i la web
// la demanen cada segon per veure'l treballar; mai engega cap navegador.
func (s *Server) handleOrdinadorLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	_, owner := ordinadorCaller(r)
	t, code, err := ordinadorTargetFor(owner, "")
	if err != nil {
		writeJSONStatus(w, code, map[string]any{"error": err.Error()})
		return
	}
	if t.c != nil {
		// Amb un ordinador remot hi ha el visor de veritat (noVNC).
		writeJSONStatus(w, http.StatusConflict, map[string]any{"error": "this account has a remote computer: use the viewer", "reason": "remote"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	img, url, title, used, ok := liveShot()
	out := map[string]any{"live": ok, "mode": "local"}
	if !used.IsZero() {
		out["usedAt"] = used.UTC().Format(time.RFC3339)
	}
	if ok {
		out["url"], out["title"] = url, title
		out["image"] = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)
	}
	writeJSON(w, out)
}
