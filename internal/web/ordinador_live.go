package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"gregal/internal/tools"
)

// liveSettle és quant s'espera després d'una acció abans de la captura.
var liveSettle = 300 * time.Millisecond

// liveShot, liveAct i liveOwner són variables perquè els tests no hagin
// d'engegar cap navegador.
var (
	liveShot  = tools.LiveShot
	liveAct   = tools.LiveAct
	liveOwner = func() string { return tools.BrowserOwnerKey(tools.LiveScope()) }
)

// handleOrdinadorLive és GET /api/ordinador/live: en mode local (l'ordinador
// de Gregal és el navegador d'aquest servidor, sense VM ni noVNC), una captura
// actual de la pestanya on treballa, amb l'adreça i el títol. El mòbil i la web
// la demanen cada segon per veure'l treballar; mai engega cap navegador.
//
// POST /api/ordinador/live {kind, x, y, text, key, dir} és la persona fent-lo
// servir des del visor (un toc, text, Enter, desplaçar, enrere): pren el
// control (Gregal s'espera) i el renova a cada acció. El text no surt mai als
// logs. Torna el mateix que el GET, amb la captura de després.
func (s *Server) handleOrdinadorLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
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
	// El navegador en directe és el de l'últim que l'ha fet servir: només el
	// seu propietari el veu i el toca.
	if owner := liveOwner(); owner != "" && owner != t.key {
		writeJSON(w, map[string]any{"live": false, "mode": "local"})
		return
	}
	if r.Method == http.MethodPost {
		var in tools.LiveInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "bad request"})
			return
		}
		// Pren (o renova) el control: l'agent no hi actua mentre el tens.
		s.ordinadorLeases().Take(t.key, "viewer", "")
		if err := liveAct(in); err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, tools.ErrNoLiveBrowser) {
				code = http.StatusConflict
			}
			writeJSONStatus(w, code, map[string]any{"error": err.Error()})
			return
		}
		time.Sleep(liveSettle)
	}
	img, url, title, used, ok := liveShot()
	out := map[string]any{"live": ok, "mode": "local", "controlOwner": "none"}
	if s.ordinadorLeases().Get(t.key) != nil {
		out["controlOwner"] = "user"
	}
	if !used.IsZero() {
		out["usedAt"] = used.UTC().Format(time.RFC3339)
	}
	if ok {
		out["url"], out["title"] = url, title
		out["image"] = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)
	}
	writeJSON(w, out)
}
