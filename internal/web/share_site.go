package web

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/tema"
)

// Webs publicades: el que l'agent ha construït, servit amb un enllaç estable.
//
// És el mateix sistema que els enllaços de converses (share.go): les
// metadades a <data>/shares/<id>.json amb kind "site", l'id de 160 bits,
// la llista a GET /api/shares i la revocació a DELETE /api/shares/{id}.
// Els fitxers són una CÒPIA a <data>/shares/<id>/: editar el projecte
// després no canvia res fins que es torna a publicar (amb el mateix id, i
// llavors l'enllaç no canvia).
//
// POST /api/shares/site publica (o republica) un HTML, un markdown o una
// carpeta del workspace. GET /share/<id>/<camí> ho serveix sense token,
// amb la capçalera CSP sandbox: els scripts de la web corren en un origen
// opac, de manera que no poden llegir el token del web de Gregal (que viu
// al localStorage d'aquest mateix origen) ni cridar l'API amb les seves
// credencials.

const (
	siteMaxBytes     = 20 << 20 // tota la web
	siteMaxFileBytes = 10 << 20 // un sol fitxer
	siteMaxFiles     = 1000
	siteMaxPerOwner  = 100
	siteMaxDays      = 365
)

// siteCSP: la web pot fer el que faria a qualsevol hosting (scripts,
// formularis, recursos externs), però des d'un origen opac.
const siteCSP = "sandbox allow-scripts allow-forms allow-popups allow-modals allow-downloads; frame-ancestors 'none'; base-uri 'self'"

// publicBase és l'adreça pública del servidor (l'instal·lador amb domini
// la posa a GREGAL_PUBLIC_URL). Buida: el client fa servir la seva.
func publicBase() string {
	b := strings.TrimRight(strings.TrimSpace(os.Getenv("GREGAL_PUBLIC_URL")), "/")
	if !strings.HasPrefix(b, "https://") && !strings.HasPrefix(b, "http://") {
		return ""
	}
	return b
}

func siteDir(id string) string { return filepath.Join(sharesDir(), id) }

func siteExpired(m shareMeta) bool {
	return m.Expires != nil && time.Now().After(*m.Expires)
}

// siteFile és un fitxer de la instantània, amb el camí relatiu amb barres.
type siteFile struct {
	rel  string
	data []byte
}

// handleShareSite: POST publica o republica una web.
func (s *Server) handleShareSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		// ID d'una publicació pròpia: la republica al mateix enllaç.
		ID string `json:"id"`
		// Kind html|markdown amb Content, o Dir: una carpeta del workspace.
		Kind    string `json:"kind"`
		Content string `json:"content"`
		Dir     string `json:"dir"`
		Title   string `json:"title"`
		// ExpiresDays: 0 = no caduca; si no, 1-365 dies des d'ara.
		ExpiresDays int `json:"expires_days"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ExpiresDays < 0 || req.ExpiresDays > siteMaxDays {
		http.Error(w, fmt.Sprintf("expires_days must be 0-%d", siteMaxDays), http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(req.Title)
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	files, err := s.siteFiles(req.Kind, req.Content, req.Dir, title)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, ErrForaArrel) {
			code = http.StatusForbidden
		}
		http.Error(w, err.Error(), code)
		return
	}
	owner := s.usuari()
	now := time.Now().UTC()
	m := shareMeta{Kind: "site", Owner: owner, Created: now}
	if req.ID != "" {
		old, err := readShareMeta(req.ID)
		if err != nil || old.Owner != owner || old.Kind != "site" {
			http.NotFound(w, r)
			return
		}
		m = old
	} else {
		n := 0
		for _, x := range listShares(owner) {
			if x.Kind == "site" {
				n++
			}
		}
		if n >= siteMaxPerOwner {
			http.Error(w, fmt.Sprintf("at most %d published sites: unpublish one first", siteMaxPerOwner), http.StatusConflict)
			return
		}
		if m.ID, err = newShareID(); err != nil {
			http.Error(w, "random source unavailable", http.StatusInternalServerError)
			return
		}
	}
	m.Title = title
	if m.Title == "" {
		m.Title = "Web"
	}
	m.Updated = &now
	m.Expires = nil
	if req.ExpiresDays > 0 {
		e := now.Add(time.Duration(req.ExpiresDays) * 24 * time.Hour)
		m.Expires = &e
	}
	m.Files, m.Bytes = len(files), 0
	for _, f := range files {
		m.Bytes += len(f.data)
	}
	if err := writeSite(m, files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, siteResponse(m))
}

// siteResponse: path sempre; url només si el servidor sap la seva adreça
// pública (si no, el client hi posa la que fa servir ell).
func siteResponse(m shareMeta) map[string]any {
	p := "/share/" + m.ID + "/"
	out := map[string]any{"id": m.ID, "kind": "site", "path": p, "url": "", "public": false,
		"title": m.Title, "created": m.Created, "updated": m.Updated, "expires": m.Expires,
		"files": m.Files, "bytes": m.Bytes}
	if b := publicBase(); b != "" {
		out["url"], out["public"] = b+p, true
	}
	return out
}

// siteFiles arma la instantània en memòria abans de tocar el disc.
func (s *Server) siteFiles(kind, content, dir, title string) ([]siteFile, error) {
	if strings.TrimSpace(dir) != "" {
		if content != "" {
			return nil, errors.New("send content or dir, not both")
		}
		return s.siteFilesFromDir(dir)
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("nothing to publish")
	}
	if len(content) > siteMaxFileBytes {
		return nil, fmt.Errorf("content is larger than %d MB", siteMaxFileBytes>>20)
	}
	switch kind {
	case "html":
		return []siteFile{{"index.html", []byte(content)}}, nil
	case "markdown":
		t := html.EscapeString(title)
		if t == "" {
			t = "Document"
		}
		page := "<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + t +
			"</title><style>body{margin:0 auto;max-width:760px;padding:28px;color:#1F2433;background:#fff;font:16px/1.65 system-ui,sans-serif}pre{overflow:auto;background:#EEF0F5;padding:14px}table{border-collapse:collapse;width:100%}td,th{border:1px solid #D5DAE4;padding:8px}a{color:#4F5BE8}img{max-width:100%}</style></head><body>" +
			Markdown(content, tema.Per("clar")) + "</body></html>"
		return []siteFile{{"index.html", []byte(page)}}, nil
	}
	return nil, errors.New("kind must be html or markdown")
}

// siteFilesFromDir copia una carpeta del workspace (els mateixos permisos
// que /api/file). Sense enllaços simbòlics ni carpetes ocultes (.git, .env).
func (s *Server) siteFilesFromDir(dir string) ([]siteFile, error) {
	root, err := s.resolveFilePath(dir)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, errors.New("dir is not a folder")
	}
	var files []siteFile
	total := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root && strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > siteMaxFileBytes {
			return fmt.Errorf("%s is larger than %d MB", d.Name(), siteMaxFileBytes>>20)
		}
		if len(files) >= siteMaxFiles {
			return fmt.Errorf("more than %d files", siteMaxFiles)
		}
		total += int(info.Size())
		if total > siteMaxBytes {
			return fmt.Errorf("the site is larger than %d MB", siteMaxBytes>>20)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files = append(files, siteFile{filepath.ToSlash(rel), data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.rel == "index.html" {
			return files, nil
		}
	}
	return nil, errors.New("the folder has no index.html")
}

// writeSite escriu la instantània nova al costat i la canvia per la vella:
// qui obre l'enllaç mentre es republica veu la vella o la nova, mai mitja.
func writeSite(m shareMeta, files []siteFile) error {
	sharesMu.Lock()
	defer sharesMu.Unlock()
	if err := os.MkdirAll(sharesDir(), 0o700); err != nil {
		return err
	}
	b := make([]byte, 6)
	_, _ = cryptorand.Read(b)
	tmp := siteDir(m.ID) + ".new-" + hex.EncodeToString(b)
	for _, f := range files {
		dst := filepath.Join(tmp, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			os.RemoveAll(tmp)
			return err
		}
		if err := os.WriteFile(dst, f.data, 0o600); err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	old := siteDir(m.ID) + ".old-" + hex.EncodeToString(b)
	if _, err := os.Stat(siteDir(m.ID)); err == nil {
		if err := os.Rename(siteDir(m.ID), old); err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, siteDir(m.ID)); err != nil {
		_ = os.Rename(old, siteDir(m.ID))
		os.RemoveAll(tmp)
		return err
	}
	os.RemoveAll(old)
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sharesDir(), m.ID+".json"), raw, 0o600)
}

// removeSite esborra una web publicada (metadades primer: l'enllaç mor).
func removeSite(id string) error {
	err := os.Remove(filepath.Join(sharesDir(), id+".json"))
	_ = os.RemoveAll(siteDir(id))
	return err
}

// handleShareSitePage serveix un fitxer d'una web publicada, sense token.
func handleShareSitePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := readShareMeta(id)
	if err != nil || m.Kind != "site" {
		http.NotFound(w, r)
		return
	}
	if siteExpired(m) {
		sharesMu.Lock()
		_ = removeSite(id)
		sharesMu.Unlock()
		http.NotFound(w, r)
		return
	}
	rel := r.PathValue("path")
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	clean := path.Clean("/" + rel)[1:]
	if clean == "" || strings.Contains(clean, "\\") || strings.HasPrefix(clean, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(siteDir(id), filepath.FromSlash(clean))
	info, err := os.Lstat(full)
	if err == nil && info.IsDir() {
		http.Redirect(w, r, "/share/"+id+"/"+clean+"/", http.StatusFound)
		return
	}
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(path.Ext(clean))
	if ct == "" {
		ct = http.DetectContentType(raw)
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("Content-Security-Policy", siteCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Cache-Control", "no-cache")
	_, _ = w.Write(raw)
}
