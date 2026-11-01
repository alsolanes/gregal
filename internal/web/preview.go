package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"html"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Previsualitzar el que el vent ha fet.
//
// Per què cal: fins ara, quan el xat demanava «serveix-me la web», el vent
// improvisava `python3 -m http.server 3xxxx` amb un `&`. Aquell servidor mor
// quan acaba l'ordre (el grup de processos es tanca), el port és diferent a
// cada intent i el tap final no hi és: la URL que dona el vent deixa de
// respondre abans que la persona l'escrigui. I al mòbil encara és pitjor
// perquè la URL no és ni tan sols fàcil de teclejar.
//
// Aquí el servidor és el propi Gregal, que ja escolta i no mor. La carpeta de
// treball es publica en lectura sota /p/<clau>/, on la clau és un HMAC del
// token: la URL es pot obrir al navegador del telèfon (sense capçalera) i no
// és endevinable si no es té el token.
const previewPrefix = "/p/"

// previewSig és la clau de la previsualització. Surt del token del servidor
// perquè canviar el token invalidï els enllaços antics.
func (s *Server) previewSig() string {
	secret := s.Token
	if secret == "" {
		// Mode local sense token: la clau ha de ser estable entre arrencades
		// però no endevinable des de fora.
		secret = "gregal-local-preview|" + s.previewRoot()
	}
	sum := sha256.Sum256([]byte(secret + "|preview|v1"))
	return hex.EncodeToString(sum[:])[:16]
}

// previewBase és el camí arrel de la previsualització (sense host).
func (s *Server) previewBase() string { return previewPrefix + s.previewSig() + "/" }

// previewOK comprova la clau de l'enllaç.
func (s *Server) previewOK(r *http.Request) (string, bool) {
	rest := strings.TrimPrefix(r.URL.Path, previewPrefix)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		got, rel := rest[:i], rest[i+1:]
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.previewSig())) == 1 {
			return rel, true
		}
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(rest), []byte(s.previewSig())) == 1 {
		return "", true
	}
	return "", false
}

// previewRoot és la carpeta que es publica: la de treball del servidor.
func (s *Server) previewRoot() string {
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	if root == "" {
		root, _ = os.Getwd()
	}
	return root
}

// previewRootSlug és una carpeta publicada amb el seu nom curt a la URL.
type previewRootSlug struct {
	Slug string // «projectes», «carpeta»
	Dir  string
}

// previewRoots són les carpetes que es publiquen: la de projectes dels vents i
// la carpeta de treball de la sessió (si són la mateixa, una sola entrada).
// Així una web feta a ~/gregal/projectes/joc i una feina sobre el repo es
// poden veure totes dues des de la mateixa pantalla.
func (s *Server) previewRoots() []previewRootSlug {
	var out []previewRootSlug
	vist := map[string]bool{}
	add := func(slug, dir string) {
		if dir == "" || vist[dir] {
			return
		}
		vist[dir] = true
		out = append(out, previewRootSlug{Slug: slug, Dir: dir})
	}
	add("projectes", asseguraProjectes())
	add("carpeta", s.previewRoot())
	return out
}

// previewPick resol una ruta relativa dins les carpetes publicades. El primer
// segment pot ser el nom de la carpeta («projectes/joc/index.html»); si no ho
// és, es prova a totes (enllaços antics, sense nom).
func (s *Server) previewPick(rel string) (string, bool) {
	roots := s.previewRoots()
	neta := strings.Trim(path.Clean("/"+strings.TrimSpace(rel)), "/")
	segs := strings.Split(neta, "/")
	for _, r := range roots {
		if segs[0] == r.Slug {
			rest := strings.Join(segs[1:], "/")
			return previewResolve(r.Dir, rest)
		}
	}
	for _, r := range roots {
		if full, ok := previewResolve(r.Dir, neta); ok {
			if _, err := os.Stat(full); err == nil {
				return full, true
			}
		}
	}
	// Que no existeixi encara no vol dir que la ruta sigui dolenta: es resol
	// a la primera carpeta perquè la resposta sigui un 404 net.
	return previewResolve(roots0(roots), neta)
}

func roots0(roots []previewRootSlug) string {
	if len(roots) == 0 {
		return ""
	}
	return roots[0].Dir
}

// previewResolve tradueix una ruta relativa a ruta real dins la carpeta, o
// diu que no. Mai surt de la carpeta i mai ensenya el que no toca.
func previewResolve(root, rel string) (string, bool) {
	clean := path.Clean("/" + strings.TrimSpace(rel))
	if clean == "/" {
		return root, true
	}
	for _, seg := range strings.Split(strings.TrimPrefix(clean, "/"), "/") {
		// Res de punt (amagats: .git, .env, .gregal) ni dependències.
		if strings.HasPrefix(seg, ".") || seg == "node_modules" || seg == "" {
			return "", false
		}
	}
	full := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	absRoot, err1 := filepath.Abs(root)
	absFull, err2 := filepath.Abs(full)
	if err1 != nil || err2 != nil {
		return "", false
	}
	// Les dues bandes es resolen igual: a Android /data/user/0 és un enllaç a
	// /data/data, i comparar el costat resolt amb l'escrit tombava qualsevol
	// fitxer (404 al mòbil i 200 al servidor, el pitjor dels mons).
	if real, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = real
	}
	if real, err := filepath.EvalSymlinks(absFull); err == nil {
		absFull = real
	}
	if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

// handlePreview serveix /p/<clau>/<ruta>: fitxers de la carpeta de treball i,
// a l'arrel i a cada carpeta, un índex navegable.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	rel, ok := s.previewOK(r)
	if !ok {
		// Sense dir si la clau existeix: 404 i prou.
		http.NotFound(w, r)
		return
	}
	full, ok := s.previewPick(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		s.previewIndex(w, r, rel, full)
		return
	}
	// Els fitxers sempre frescos: mentre es prova una web, una còpia vella és
	// pitjor que cap còpia.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(full))); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, filepath.Base(full), info.ModTime(), f)
}

// previewIndex pinta un índex senzill de la carpeta (o de la carpeta pare).
func (s *Server) previewIndex(w http.ResponseWriter, r *http.Request, rel, full string) {
	var b strings.Builder
	base := s.previewBase()
	b.WriteString("<!doctype html><meta charset=utf-8><title>Carpeta de treball</title>")
	b.WriteString("<meta name=viewport content='width=device-width,initial-scale=1'>")
	b.WriteString("<style>body{font:16px/1.5 ui-sans-serif,system-ui;margin:0;padding:20px;background:#12141a;color:#e8e6df}")
	b.WriteString("a{color:#8fd0ff;text-decoration:none}a:hover{text-decoration:underline}")
	b.WriteString("li{margin:6px 0}h2{font-size:15px;color:#8a8f98;margin:22px 0 6px;text-transform:uppercase;letter-spacing:.06em}")
	b.WriteString("code{color:#8a8f98}header{margin-bottom:16px;font-weight:600}</style>")

	neta := strings.Trim(path.Clean("/"+strings.TrimSpace(rel)), "/")
	if neta == "" {
		// Arrel: totes les carpetes publicades, una darrere l'altra.
		b.WriteString("<header>Gregal · carpeta de treball</header>")
		for _, root := range s.previewRoots() {
			b.WriteString("<h2>" + html.EscapeString(root.Slug) + "</h2><ul>")
			s.writePreviewList(&b, base+root.Slug+"/", root.Dir)
			b.WriteString("</ul>")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(b.String()))
		return
	}

	ents, err := os.ReadDir(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sort.Slice(ents, func(i, j int) bool {
		di, dj := ents[i].IsDir(), ents[j].IsDir()
		if di != dj {
			return di
		}
		return ents[i].Name() < ents[j].Name()
	})
	b.WriteString("<header>" + html.EscapeString(neta) + "</header><ul>")
	up := path.Dir(neta)
	if up == "." {
		up = ""
	}
	b.WriteString("<li><a href='" + base + up + "'>↑ ..</a></li>")
	s.writePreviewEntries(&b, base+neta+"/", ents)
	b.WriteString("</ul>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(b.String()))
}

// writePreviewList pinta el contingut d'una carpeta (per a l'arrel múltiple).
func (s *Server) writePreviewList(b *strings.Builder, linkBase, dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) == 0 {
		b.WriteString("<li><code>buit</code></li>")
		return
	}
	sort.Slice(ents, func(i, j int) bool {
		di, dj := ents[i].IsDir(), ents[j].IsDir()
		if di != dj {
			return di
		}
		return ents[i].Name() < ents[j].Name()
	})
	s.writePreviewEntries(b, linkBase, ents)
}

// writePreviewEntries escriu fitxers i carpetes d'un directori.
func (s *Server) writePreviewEntries(b *strings.Builder, linkBase string, ents []os.DirEntry) {
	shown := 0
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		link := linkBase + name
		if e.IsDir() {
			b.WriteString("<li>📁 <a href='" + link + "/'>" + html.EscapeString(name) + "/</a></li>")
			shown++
			continue
		}
		size := ""
		if info, err := e.Info(); err == nil {
			size = " <code>" + strconv.FormatInt(info.Size(), 10) + " B</code>"
		}
		b.WriteString("<li>📄 <a href='" + link + "'>" + html.EscapeString(name) + "</a>" + size + "</li>")
		shown++
	}
	if shown == 0 {
		b.WriteString("<li><code>buit</code></li>")
	}
}

// previewWeb és una web trobada a la carpeta de treball.
type previewWeb struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
}

// previewWebs busca HTML de la carpeta de treball (fins a tres nivells) per
// oferir-lo a l'app: index.html primer, després la resta per data.
func previewWebs(root string) []previewWeb {
	var out []previewWeb
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		nivell := 0
		if relSlash != "." {
			nivell = strings.Count(relSlash, "/") + 1
		}
		if d.IsDir() {
			name := d.Name()
			if relSlash != "." {
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "vendor" {
					return fs.SkipDir
				}
				if nivell > 3 {
					return fs.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".html" && ext != ".htm" {
			return nil
		}
		if nivell > 3 {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		out = append(out, previewWeb{
			Path:  relSlash,
			Name:  relSlash,
			Size:  info.Size(),
			Mtime: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		bi, bj := filepath.Base(out[i].Path) == "index.html", filepath.Base(out[j].Path) == "index.html"
		if bi != bj {
			return bi
		}
		return out[i].Mtime > out[j].Mtime
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// handlePreviewInfo diu a l'app què pot previsualitzar i amb quina URL.
func (s *Server) handlePreviewInfo(w http.ResponseWriter, r *http.Request) {
	root := s.previewRoot()
	roots := s.previewRoots()
	var webs []previewWeb
	for _, rr := range roots {
		for _, wv := range previewWebs(rr.Dir) {
			wv.Path = rr.Slug + "/" + wv.Path
			wv.Name = filepath.Base(wv.Path)
			webs = append(webs, wv)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"root":  root,
		"roots": roots,
		"base":  s.previewBase(), // el client hi enganxa el seu host
		"webs":  webs,
		"index": s.previewBase(),
	})
}
