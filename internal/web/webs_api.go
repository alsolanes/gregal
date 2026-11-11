package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// API de les webs amb nom (B11). Tot és del compte que pregunta:
//
//	GET    /api/webs                        domini, «pot publicar» i les webs pròpies
//	POST   /api/webs                        publica {name, dir | kind+content, title, visibility, preview}
//	GET    /api/webs/{name}                 una web
//	DELETE /api/webs/{name}                 despublica i allibera el nom
//	POST   /api/webs/{name}/visibility      {visibility} sense tornar a publicar
//	POST   /api/webs/{name}/undo            torna a la versió activa d'abans
//	POST   /api/webs/{name}/activate        {version, visibility} comprova i activa una versió
//	POST   /api/webs/{name}/check           torna a comprovar l'activa
//	POST   /api/webs/{name}/open            {preview} → {url} (amb bitllet si cal sessió)
//	POST   /api/webs/publishers             {user, allow} (administrador)

func registerWebsRoutes(h *Hub, handle func(string, http.HandlerFunc)) {
	handle("/api/webs", h.route((*Server).handleWebs))
	handle("/api/webs/publishers", h.global(adminOnly((*Server).handleWebPublishers)))
	handle("/api/webs/{name}", h.route((*Server).handleWeb))
	handle("/api/webs/{name}/{action}", h.route((*Server).handleWebAction))
}

func webHTTPError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, errWebTaken):
		code = http.StatusConflict
	case errors.Is(err, errWebNotFound):
		code = http.StatusNotFound
	case errors.Is(err, errWebCannot):
		code = http.StatusForbidden
	case errors.Is(err, ErrForaArrel):
		code = http.StatusForbidden
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (s *Server) handleWebs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		base := requestBase(r)
		list := []map[string]any{}
		for _, x := range listWebs(s.remindOwner()) {
			list = append(list, s.webView(x, base))
		}
		writeJSON(w, map[string]any{"domain": s.publishDomain(), "canPublish": s.canPublish(), "sites": list})
	case http.MethodPost:
		var req struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Content    string `json:"content"`
			Dir        string `json:"dir"`
			Title      string `json:"title"`
			Visibility string `json:"visibility"`
			Preview    bool   `json:"preview"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, siteMaxFileBytes+(1<<20))).Decode(&req); err != nil {
			webHTTPError(w, errors.New("invalid json"))
			return
		}
		name := strings.ToLower(strings.TrimSpace(req.Name))
		if err := validWebName(name); err != nil {
			webHTTPError(w, err)
			return
		}
		if !s.canPublish() {
			webHTTPError(w, errWebCannot)
			return
		}
		if x := getWeb(name); x != nil && x.Owner != s.remindOwner() {
			webHTTPError(w, errWebTaken)
			return
		}
		files, err := s.siteFiles(req.Kind, req.Content, req.Dir, strings.TrimSpace(req.Title))
		if err != nil {
			webHTTPError(w, err)
			return
		}
		src := ""
		if strings.TrimSpace(req.Dir) != "" {
			src, _ = s.resolveToolPath(req.Dir)
		}
		// Des de l'app és la mateixa persona qui tria «public»: és la confirmació.
		res, err := s.webPublish(webPublishOpts{Name: name, Visibility: strings.ToLower(strings.TrimSpace(req.Visibility)),
			Title: strings.TrimSpace(req.Title), Source: src, Stage: req.Preview, Base: requestBase(r), PublicOK: true}, files)
		if err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, s.webResultJSON(res, requestBase(r)))
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) webResultJSON(res *webPublishResult, base string) map[string]any {
	out := map[string]any{"site": s.webView(res.Site, base), "version": res.Version, "receipt": s.webReceipt(res), "held": res.Held}
	if res.Check != nil {
		out["check"] = res.Check
		out["ok"] = res.Check.OK
	}
	return out
}

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	site := getWeb(name)
	if site == nil || site.Owner != s.remindOwner() {
		webHTTPError(w, errWebNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.webView(site, requestBase(r)))
	case http.MethodDelete:
		if err := s.webRemove(name); err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, map[string]any{"deleted": name})
	default:
		http.Error(w, "GET or DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWebAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	site := getWeb(name)
	if site == nil || site.Owner != s.remindOwner() {
		webHTTPError(w, errWebNotFound)
		return
	}
	var req struct {
		Visibility string `json:"visibility"`
		Version    int    `json:"version"`
		Preview    bool   `json:"preview"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req)
	base := requestBase(r)
	switch r.PathValue("action") {
	case "visibility":
		x, err := s.webSetVisibility(name, strings.ToLower(strings.TrimSpace(req.Visibility)))
		if err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, s.webView(x, base))
	case "undo":
		x, err := s.webUndo(name)
		if err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, s.webView(x, base))
	case "activate", "check":
		n := req.Version
		if r.PathValue("action") == "check" || n == 0 {
			n = site.Current
			if r.PathValue("action") == "activate" && req.Version == 0 {
				n = site.latest()
			}
		}
		if n == 0 {
			webHTTPError(w, errors.New("nothing published yet"))
			return
		}
		if !s.canPublish() {
			webHTTPError(w, errWebCannot)
			return
		}
		res, err := s.webActivate(name, n, strings.ToLower(strings.TrimSpace(req.Visibility)), true, base)
		if err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, s.webResultJSON(res, base))
	case "open":
		writeJSON(w, map[string]any{"url": s.webOpenURL(site, req.Preview, base)})
	default:
		http.NotFound(w, r)
	}
}

// webOpenURL: l'adreça per obrir la web des de l'app. Si cal la sessió del
// propietari (private o previsualització) porta un bitllet d'un sol ús.
func (s *Server) webOpenURL(site *webSite, preview bool, base string) string {
	urls := s.webURLs(site, base)
	u := urls["url"]
	if preview {
		u = urls["previewUrl"]
	}
	if u == "" {
		if preview {
			u = urls["previewPath"]
		} else {
			u = urls["path"]
		}
	}
	if preview || site.Visibility == webPrivate {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + webTicketParam + "=" + url.QueryEscape(newWebTicket(site.Owner, site.Name))
	}
	return u
}

func (s *Server) handleWebPublishers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var cfg []string
		if s.cfg != nil {
			cfg = s.cfg.Publish.Publishers
		}
		writeJSON(w, map[string]any{"config": cfg, "allow": readWebPublishers().Allow})
	case http.MethodPost:
		var req struct {
			User  string `json:"user"`
			Allow bool   `json:"allow"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil || strings.TrimSpace(req.User) == "" {
			webHTTPError(w, errors.New("user is required"))
			return
		}
		if err := setWebPublisher(strings.TrimSpace(req.User), req.Allow); err != nil {
			webHTTPError(w, err)
			return
		}
		writeJSON(w, map[string]any{"allow": readWebPublishers().Allow})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}
