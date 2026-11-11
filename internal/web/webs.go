package web

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Webs pròpies amb nom (B11): <nom>.<domini>, <nom>.lan.<domini> o /w/<nom>/.
//
// Principi: l'ordinador construeix, l'amfitrió serveix. Publicar COPIA una
// versió immutable a <dades>/webs/sites/<nom>/v<N>/ (com share_site.go), i el
// registre <dades>/webs/registry.json diu de qui és cada nom, amb quina
// visibilitat i quina versió és l'activa. Res del que es publica depèn que
// l'ordinador de la persona estigui encès.
//
//   - El primer compte que reclama un nom el té; un altre no el pot
//     sobreescriure ni esborrar.
//   - Visibilitat public | lan | private, canviable sense tornar a publicar.
//     lan només es serveix a adreces privades i mai pel túnel; private
//     demana la sessió del propietari (galeta de bitllet, webs_serve.go).
//   - Cada publicació és una versió nova; la nova només passa a ser l'activa
//     si la comprovació de l'URL real va bé (webs_check.go). «Desfés» torna
//     a la versió activa anterior a l'instant. Es conserven les 10 últimes.
//   - El domini base és configuració de la instal·lació (publish.domain o
//     GREGAL_PUBLISH_DOMAIN). Sense domini: només /w/<nom>/.

const (
	webKeepVersions = 10
	webMaxPerOwner  = 50
)

// Visibilitats vàlides.
const (
	webPublic  = "public"
	webLAN     = "lan"
	webPrivate = "private"
)

// webReserved: noms que mai pot tenir una web (són de la instal·lació).
var webReserved = map[string]bool{"www": true, "gregal": true, "api": true, "lan": true, "preview": true}

var reWebName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

var (
	errWebTaken    = errors.New("this name belongs to another account")
	errWebNotFound = errors.New("no such site")
	errWebNoUndo   = errors.New("there is no earlier version to go back to")
	errWebCannot   = errors.New("this account cannot publish sites: an administrator has to allow it")
)

// validWebName: etiqueta DNS en minúscules (a-z, 0-9, guió), no reservada.
func validWebName(name string) error {
	if !reWebName.MatchString(name) {
		return errors.New("invalid name: use 1-63 lowercase letters, digits or hyphens, not starting or ending with a hyphen")
	}
	if strings.HasPrefix(name, "xn--") {
		return errors.New("invalid name: punycode names are not allowed")
	}
	if webReserved[name] {
		return fmt.Errorf("the name %q is reserved", name)
	}
	return nil
}

func validWebVisibility(v string) bool { return v == webPublic || v == webLAN || v == webPrivate }

type webCheck struct {
	At       time.Time `json:"at"`
	OK       bool      `json:"ok"`
	URL      string    `json:"url"`
	Version  int       `json:"version"`
	Status   int       `json:"status,omitempty"`
	Links    int       `json:"links,omitempty"`
	Problems []string  `json:"problems,omitempty"`
}

type webVersion struct {
	N       int       `json:"n"`
	Created time.Time `json:"created"`
	Files   int       `json:"files"`
	Bytes   int       `json:"bytes"`
	Title   string    `json:"title,omitempty"`
	// Source és la carpeta d'origen (només la veu el propietari).
	Source string    `json:"source,omitempty"`
	Check  *webCheck `json:"check,omitempty"`
}

type webSite struct {
	Name       string `json:"name"`
	Owner      string `json:"owner"`
	Visibility string `json:"visibility"`
	Kind       string `json:"kind"` // static
	Title      string `json:"title,omitempty"`
	// Current és la versió activa (0 = encara cap).
	Current int `json:"current"`
	// History són les versions que han estat actives, en ordre: «desfés»
	// treu l'última i torna a la d'abans.
	History  []int        `json:"history,omitempty"`
	Versions []webVersion `json:"versions,omitempty"`
	Next     int          `json:"next"`
	// PublicConfirmed: la persona ja ha dit que sí a fer públic aquest nom.
	PublicConfirmed bool      `json:"public_confirmed,omitempty"`
	LastCheck       *webCheck `json:"last_check,omitempty"`
	Created         time.Time `json:"created"`
	Updated         time.Time `json:"updated"`
}

func (w *webSite) version(n int) *webVersion {
	for i := range w.Versions {
		if w.Versions[i].N == n {
			return &w.Versions[i]
		}
	}
	return nil
}

func (w *webSite) latest() int {
	n := 0
	for _, v := range w.Versions {
		if v.N > n {
			n = v.N
		}
	}
	return n
}

// --- registre ---

var websMu sync.Mutex

func websDir() string          { return filepath.Join(serverDataDir(), "webs") }
func websRegistryPath() string { return filepath.Join(websDir(), "registry.json") }
func webVersionDir(name string, n int) string {
	return filepath.Join(websDir(), "sites", name, fmt.Sprintf("v%d", n))
}

func loadWebsLocked() map[string]*webSite {
	out := map[string]*webSite{}
	raw, err := os.ReadFile(websRegistryPath())
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func saveWebsLocked(m map[string]*webSite) error {
	if err := os.MkdirAll(websDir(), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(websRegistryPath(), raw)
}

// getWeb torna una còpia del registre d'un nom (nil si no hi és).
func getWeb(name string) *webSite {
	websMu.Lock()
	defer websMu.Unlock()
	w := loadWebsLocked()[name]
	if w == nil {
		return nil
	}
	cp := *w
	cp.History = append([]int(nil), w.History...)
	cp.Versions = append([]webVersion(nil), w.Versions...)
	return &cp
}

// updateWeb aplica fn al nom sota el bloqueig i desa. fn rep nil si no hi és.
func updateWeb(name string, fn func(m map[string]*webSite, w *webSite) (*webSite, error)) (*webSite, error) {
	websMu.Lock()
	defer websMu.Unlock()
	m := loadWebsLocked()
	w, err := fn(m, m[name])
	if err != nil {
		return nil, err
	}
	if w == nil {
		delete(m, name)
	} else {
		w.Updated = time.Now().UTC()
		m[name] = w
	}
	if err := saveWebsLocked(m); err != nil {
		return nil, err
	}
	if w == nil {
		return nil, nil
	}
	cp := *w
	return &cp, nil
}

func listWebs(owner string) []*webSite {
	websMu.Lock()
	defer websMu.Unlock()
	var out []*webSite
	for _, w := range loadWebsLocked() {
		if w.Owner == owner {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// --- configuració de la instal·lació ---

// publishDomainPort llegeix publish.domain: un domini (example.org) amb un
// port opcional (example.org:8080) per a instal·lacions sense proxy a 80/443.
func (s *Server) publishDomainPort() (string, string) {
	d := strings.TrimSpace(os.Getenv("GREGAL_PUBLISH_DOMAIN"))
	if d == "" && s != nil && s.cfg != nil {
		d = strings.TrimSpace(s.cfg.Publish.Domain)
	}
	d = strings.ToLower(strings.Trim(d, ". "))
	port := ""
	if h, p, err := net.SplitHostPort(d); err == nil {
		if n, err := strconv.Atoi(p); err != nil || n <= 0 || n > 65535 {
			return "", ""
		}
		d, port = strings.Trim(h, "."), ":"+p
	}
	if d == "" || strings.ContainsAny(d, "/:@ \\[]") {
		return "", ""
	}
	return d, port
}

// publishDomain és el domini base (sense punt final ni port), o "".
func (s *Server) publishDomain() string {
	d, _ := s.publishDomainPort()
	return d
}

func (s *Server) publishScheme() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("GREGAL_PUBLISH_SCHEME"))); v == "http" || v == "https" {
		return v
	}
	if s != nil && s.cfg != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Publish.Scheme), "http") {
		return "http"
	}
	return "https"
}

// --- capacitat «pot publicar» ---

type webPublishers struct {
	Allow []string `json:"allow"`
}

func webPublishersPath() string { return filepath.Join(websDir(), "publishers.json") }

func readWebPublishers() webPublishers {
	var p webPublishers
	raw, err := os.ReadFile(webPublishersPath())
	if err == nil {
		_ = json.Unmarshal(raw, &p)
	}
	return p
}

func setWebPublisher(name string, allow bool) error {
	websMu.Lock()
	defer websMu.Unlock()
	p := readWebPublishers()
	var out []string
	for _, n := range p.Allow {
		if n != name {
			out = append(out, n)
		}
	}
	if allow {
		out = append(out, name)
	}
	sort.Strings(out)
	p.Allow = out
	if err := os.MkdirAll(websDir(), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(p, "", "  ")
	return writeFileAtomic(webPublishersPath(), raw)
}

// canPublish: «pot publicar» és una capacitat per compte, separada dels
// poders de servidor. Els administradors (i el mode local) la tenen; la
// resta, si el config (publish.publishers) o l'administrador la hi dona.
func (s *Server) canPublish() bool {
	u := s.user
	if u == nil || u.Tothom {
		return true
	}
	if admin, ok := s.users.IsAdmin(u.Name); ok {
		if admin {
			return true
		}
	} else if u.Admin {
		return true
	}
	if s.cfg != nil {
		for _, n := range s.cfg.Publish.Publishers {
			if n = strings.TrimSpace(n); n == "*" || n == u.Name {
				return true
			}
		}
	}
	for _, n := range readWebPublishers().Allow {
		if n == u.Name {
			return true
		}
	}
	return false
}

// --- confirmació del primer «public» (B4) ---

// webPublicGrants: la persona ha aprovat fer públic per primer cop aquest
// nom (decisió del fil). L'eina publish la consumeix un sol cop.
var webPublicGrants sync.Map // owner\x00name → time.Time

func grantWebPublic(owner, name string) {
	webPublicGrants.Store(owner+"\x00"+name, time.Now())
}

func consumeWebPublic(owner, name string) bool {
	v, ok := webPublicGrants.LoadAndDelete(owner + "\x00" + name)
	if !ok {
		return false
	}
	return time.Since(v.(time.Time)) < 30*time.Minute
}

// webNeedsPublicDecision diu si una crida de l'eina publish fa pública per
// primer cop un nom: això és una decisió que la persona confirma al fil.
// Les actualitzacions d'un nom ja confirmat, no.
func (s *Server) webNeedsPublicDecision(argsJSON string) (bool, string) {
	var a struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return false, ""
	}
	name := strings.ToLower(strings.TrimSpace(a.Name))
	if name == "" || strings.ToLower(strings.TrimSpace(a.Visibility)) != webPublic || validWebName(name) != nil {
		return false, ""
	}
	w := getWeb(name)
	if w != nil && (w.Owner != s.remindOwner() || w.PublicConfirmed) {
		// D'un altre: l'eina ja ho rebutjarà. Confirmat: és una actualització.
		return false, ""
	}
	return true, s.webPublicURL(name, "")
}

// --- operacions ---

type webPublishOpts struct {
	Name, Visibility, Title, Source string
	// Stage: només previsualització, la versió activa no canvia.
	Stage bool
	// Base és l'adreça de Gregal que veu el client (per a /w/<nom>/).
	Base string
	// PublicOK: la persona ha confirmat el primer «public» d'aquest nom.
	PublicOK bool
}

type webPublishResult struct {
	Site    *webSite
	Version int
	Check   *webCheck
	// Held: s'ha demanat public sense confirmació; ha quedat privada.
	Held bool
}

// webStage reclama el nom (si cal) i escriu la versió nova. No l'activa.
func (s *Server) webStage(o webPublishOpts, files []siteFile) (*webSite, int, error) {
	owner := s.remindOwner()
	if !s.canPublish() {
		return nil, 0, errWebCannot
	}
	if err := validWebName(o.Name); err != nil {
		return nil, 0, err
	}
	var n int
	w, err := updateWeb(o.Name, func(m map[string]*webSite, w *webSite) (*webSite, error) {
		if w != nil && w.Owner != owner {
			return nil, errWebTaken
		}
		if w == nil {
			c := 0
			for _, x := range m {
				if x.Owner == owner {
					c++
				}
			}
			if c >= webMaxPerOwner {
				return nil, fmt.Errorf("at most %d sites per account: remove one first", webMaxPerOwner)
			}
			now := time.Now().UTC()
			w = &webSite{Name: o.Name, Owner: owner, Visibility: webPrivate, Kind: "static", Created: now}
		}
		n = max(w.Next, w.latest()) + 1
		w.Next = n
		dir := webVersionDir(o.Name, n)
		if err := writeWebFiles(dir, files); err != nil {
			return nil, err
		}
		v := webVersion{N: n, Created: time.Now().UTC(), Files: len(files), Title: o.Title, Source: o.Source}
		for _, f := range files {
			v.Bytes += len(f.data)
		}
		w.Versions = append(w.Versions, v)
		if o.Title != "" {
			w.Title = o.Title
		}
		pruneWebVersions(w)
		return w, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return w, n, nil
}

// writeWebFiles escriu una versió a una carpeta temporal i la canvia de nom:
// una versió és sencera o no hi és.
func writeWebFiles(dir string, files []siteFile) error {
	b := make([]byte, 6)
	_, _ = cryptorand.Read(b)
	tmp := dir + ".new-" + hex.EncodeToString(b)
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
	_ = os.RemoveAll(dir)
	if err := os.Rename(tmp, dir); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

// pruneWebVersions conserva les 10 últimes (i sempre l'activa).
func pruneWebVersions(w *webSite) {
	sort.Slice(w.Versions, func(i, j int) bool { return w.Versions[i].N < w.Versions[j].N })
	if len(w.Versions) <= webKeepVersions {
		return
	}
	keep := map[int]bool{w.Current: true}
	for _, v := range w.Versions[len(w.Versions)-webKeepVersions:] {
		keep[v.N] = true
	}
	var out []webVersion
	for _, v := range w.Versions {
		if keep[v.N] {
			out = append(out, v)
		} else {
			_ = os.RemoveAll(webVersionDir(w.Name, v.N))
		}
	}
	w.Versions = out
	var hist []int
	for _, n := range w.History {
		if keep[n] {
			hist = append(hist, n)
		}
	}
	w.History = hist
}

// webPublish: versió nova, comprovació de l'URL real i, si va bé, activa.
func (s *Server) webPublish(o webPublishOpts, files []siteFile) (*webPublishResult, error) {
	if o.Visibility != "" && !validWebVisibility(o.Visibility) {
		return nil, errors.New("visibility must be public, lan or private")
	}
	w, n, err := s.webStage(o, files)
	if err != nil {
		return nil, err
	}
	res := &webPublishResult{Site: w, Version: n}
	if o.Stage {
		return res, nil
	}
	return s.webActivate(o.Name, n, o.Visibility, o.PublicOK, o.Base)
}

// webActivate comprova la versió n a l'URL real i, si va bé, la fa activa
// (i aplica la visibilitat demanada). Si falla, res no canvia.
func (s *Server) webActivate(name string, n int, vis string, publicOK bool, base string) (*webPublishResult, error) {
	owner := s.remindOwner()
	w := getWeb(name)
	if w == nil || w.Owner != owner {
		return nil, errWebNotFound
	}
	if w.version(n) == nil {
		return nil, fmt.Errorf("version %d does not exist", n)
	}
	if vis == "" {
		vis = w.Visibility
	}
	if !validWebVisibility(vis) {
		return nil, errors.New("visibility must be public, lan or private")
	}
	held := false
	if vis == webPublic && !w.PublicConfirmed && !publicOK {
		vis, held = webPrivate, true
	}
	chk := s.webCheckVersion(name, n, vis, base)
	w, err := updateWeb(name, func(m map[string]*webSite, w *webSite) (*webSite, error) {
		if w == nil || w.Owner != owner {
			return nil, errWebNotFound
		}
		if v := w.version(n); v != nil {
			v.Check = chk
		}
		w.LastCheck = chk
		if publicOK {
			w.PublicConfirmed = true
		}
		if chk.OK {
			if w.Current != n {
				w.Current = n
				w.History = append(w.History, n)
			}
			w.Visibility = vis
		}
		return w, nil
	})
	if err != nil {
		return nil, err
	}
	s.emitWeb(w)
	return &webPublishResult{Site: w, Version: n, Check: chk, Held: held}, nil
}

// webUndo torna a la versió activa d'abans, a l'instant (ja es va comprovar).
func (s *Server) webUndo(name string) (*webSite, error) {
	owner := s.remindOwner()
	w, err := updateWeb(name, func(m map[string]*webSite, w *webSite) (*webSite, error) {
		if w == nil || w.Owner != owner {
			return nil, errWebNotFound
		}
		hist := append([]int(nil), w.History...)
		for len(hist) > 0 && hist[len(hist)-1] != w.Current {
			hist = hist[:len(hist)-1]
		}
		if len(hist) > 0 {
			hist = hist[:len(hist)-1]
		}
		for len(hist) > 0 {
			prev := hist[len(hist)-1]
			if _, err := os.Stat(webVersionDir(name, prev)); err == nil && w.version(prev) != nil {
				w.Current = prev
				w.History = hist
				return w, nil
			}
			hist = hist[:len(hist)-1]
		}
		return nil, errWebNoUndo
	})
	if err == nil {
		s.emitWeb(w)
	}
	return w, err
}

// webSetVisibility canvia qui la pot veure, sense tornar a publicar. Fer-la
// pública des de la llista és un gest de la persona: compta com a
// confirmació del primer «public».
func (s *Server) webSetVisibility(name, vis string) (*webSite, error) {
	if !validWebVisibility(vis) {
		return nil, errors.New("visibility must be public, lan or private")
	}
	owner := s.remindOwner()
	if !s.canPublish() {
		return nil, errWebCannot
	}
	w, err := updateWeb(name, func(m map[string]*webSite, w *webSite) (*webSite, error) {
		if w == nil || w.Owner != owner {
			return nil, errWebNotFound
		}
		w.Visibility = vis
		if vis == webPublic {
			w.PublicConfirmed = true
		}
		return w, nil
	})
	if err == nil {
		s.emitWeb(w)
	}
	return w, err
}

// webRemove despublica i allibera el nom (només el propietari).
func (s *Server) webRemove(name string) error {
	owner := s.remindOwner()
	_, err := updateWeb(name, func(m map[string]*webSite, w *webSite) (*webSite, error) {
		if w == nil || w.Owner != owner {
			return nil, errWebNotFound
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(websDir(), "sites", name))
	return nil
}

// --- URLs ---

// webPublicURL: on es veu la web segons la visibilitat. Amb domini,
// <nom>.<domini> (o <nom>.lan.<domini>); sense, <base>/w/<nom>/.
func (s *Server) webPublicURL(name, vis string) string {
	if d, port := s.publishDomainPort(); d != "" {
		if vis == webLAN {
			return "http://" + name + ".lan." + d + port + "/"
		}
		return s.publishScheme() + "://" + name + "." + d + port + "/"
	}
	return ""
}

func (s *Server) webPreviewURL(name string) string {
	if d, port := s.publishDomainPort(); d != "" {
		return s.publishScheme() + "://" + name + ".preview." + d + port + "/"
	}
	return ""
}

// webURLs: url (la de la visibilitat), path (/w/<nom>/, sempre) i preview.
func (s *Server) webURLs(w *webSite, base string) map[string]string {
	if base == "" {
		base = publicBase()
	}
	out := map[string]string{
		"url":         s.webPublicURL(w.Name, w.Visibility),
		"path":        "/w/" + w.Name + "/",
		"previewPath": "/wp/" + w.Name + "/",
		"previewUrl":  s.webPreviewURL(w.Name),
	}
	if out["url"] == "" && base != "" {
		out["url"] = base + out["path"]
	}
	if out["previewUrl"] == "" && base != "" {
		out["previewUrl"] = base + out["previewPath"]
	}
	return out
}

// webHostLabel és el que es mostra a la persona: aliments.example.org o /w/aliments/.
func (s *Server) webHostLabel(w *webSite) string {
	if u := s.webPublicURL(w.Name, w.Visibility); u != "" {
		if p, err := url.Parse(u); err == nil {
			return p.Host
		}
	}
	return "/w/" + w.Name + "/"
}

// webState resumeix l'estat per a la llista: live, draft o check_failed.
func webState(w *webSite) string {
	switch {
	case w.LastCheck != nil && !w.LastCheck.OK:
		return "check_failed"
	case w.Current == 0:
		return "draft"
	}
	return "live"
}

// webView és el que veu el propietari (mai Source d'un altre).
func (s *Server) webView(w *webSite, base string) map[string]any {
	vers := []map[string]any{}
	for i := len(w.Versions) - 1; i >= 0; i-- {
		v := w.Versions[i]
		item := map[string]any{"n": v.N, "created": v.Created, "files": v.Files, "bytes": v.Bytes, "active": v.N == w.Current}
		if v.Check != nil {
			item["checked"] = v.Check.OK
		}
		vers = append(vers, item)
	}
	urls := s.webURLs(w, base)
	canUndo := false
	for _, n := range w.History {
		if n != w.Current {
			canUndo = true
		}
	}
	return map[string]any{
		"name": w.Name, "title": w.Title, "visibility": w.Visibility, "kind": w.Kind,
		"version": w.Current, "latest": w.latest(), "versions": vers, "state": webState(w),
		"check": w.LastCheck, "canUndo": canUndo && len(w.History) > 1, "publicConfirmed": w.PublicConfirmed,
		"url": urls["url"], "path": urls["path"], "previewUrl": urls["previewUrl"], "previewPath": urls["previewPath"],
		"host": s.webHostLabel(w), "updated": w.Updated, "created": w.Created,
	}
}

// webReceipt és el rebut que entra al fil: «Publicat a … · versió 4 · comprovat».
func (s *Server) webReceipt(r *webPublishResult) string {
	w := r.Site
	host := s.webHostLabel(w)
	if r.Check == nil {
		return s.tr(fmt.Sprintf("Versió %d preparada per previsualitzar a %s", r.Version, s.webURLs(w, "")["previewPath"]),
			fmt.Sprintf("Version %d staged for preview at %s", r.Version, s.webURLs(w, "")["previewPath"]))
	}
	if !r.Check.OK {
		return s.tr(fmt.Sprintf("No publicat: la comprovació de %s ha fallat (versió %d) · la versió activa continua sent la %d · %s", r.Check.URL, r.Version, w.Current, strings.Join(r.Check.Problems, "; ")),
			fmt.Sprintf("Not published: the check of %s failed (version %d) · the active version is still %d · %s", r.Check.URL, r.Version, w.Current, strings.Join(r.Check.Problems, "; ")))
	}
	out := s.tr(fmt.Sprintf("Publicat a %s · versió %d · comprovat", host, w.Current),
		fmt.Sprintf("Published at %s · version %d · checked", host, w.Current))
	if w.Visibility != webPublic {
		out += " · " + w.Visibility
	}
	if r.Held {
		out += s.tr(" · és privada fins que confirmis que ha de ser pública", " · it stays private until you confirm it should be public")
	}
	return out
}

// webArgName treu el nom dels arguments de l'eina publish.
func webArgName(argsJSON string) string {
	var a struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return strings.ToLower(strings.TrimSpace(a.Name))
}

// --- regla d'ingress comodí ---

var webIngress struct {
	sync.Mutex
	domain, tunnel string
}

func (s *Server) publishTunnel() string {
	if v := strings.TrimSpace(os.Getenv("GREGAL_PUBLISH_TUNNEL")); v != "" {
		return v
	}
	if s != nil && s.cfg != nil {
		return strings.TrimSpace(s.cfg.Publish.Tunnel)
	}
	return ""
}

func setWebIngress(domain, tunnel string) {
	webIngress.Lock()
	webIngress.domain, webIngress.tunnel = domain, tunnel
	webIngress.Unlock()
}

// webIngressRule: *.<domini> → aquest Gregal, al túnel amb nom configurat.
// Una web nova no toca ni DNS ni Cloudflare: el registre DNS comodí i
// aquesta regla es posen un sol cop (instal·lació).
func webIngressRule(tunnel string) (tunnelRecord, bool) {
	webIngress.Lock()
	d, t := webIngress.domain, webIngress.tunnel
	webIngress.Unlock()
	if d == "" || t == "" || gregalListenPort == 0 || (tunnel != "" && tunnel != t) {
		return tunnelRecord{}, false
	}
	return tunnelRecord{ID: "webs", Name: "webs", Hostname: "*." + d, Tunnel: t, Want: true,
		Target: "http://127.0.0.1:" + strconv.Itoa(gregalListenPort)}, true
}
