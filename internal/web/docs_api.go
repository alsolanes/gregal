package web

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gregal/internal/gdocs"
	"gregal/internal/ooxml"
	"gregal/internal/tema"
)

// handleDocExport baixa un document viu del projecte de la sessió com a
// fitxer d'Office, fet dels blocs amb el tema de la marca activa: un
// document de text en Word, una presentació en PowerPoint, un full de
// càlcul en Excel.
//
//	GET /api/docs/{id}/export?format=docx|pptx|xlsx
func (s *Server) handleDocExport(w http.ResponseWriter, r *http.Request) {
	st, d, ok := s.sessionDoc(w, r)
	if !ok {
		return
	}
	format := map[string]string{gdocs.KindDoc: "docx", gdocs.KindDeck: "pptx", gdocs.KindSheet: "xlsx"}[d.Kind]
	if f := r.URL.Query().Get("format"); f != "" && f != format {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "format: a " + d.Kind + " exports only as " + format})
		return
	}
	// Sense tema, NewDocx i NewPptx fan servir el per defecte, que és la marca activa.
	var pkg *ooxml.Package
	var data []byte
	var err error
	ctype := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	switch d.Kind {
	case gdocs.KindSheet:
		data, err = sheetXlsx(d)
		ctype = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case gdocs.KindDeck:
		spec := ooxml.DeckSpec{Title: d.Title}
		for _, b := range d.Blocks {
			spec.Slides = append(spec.Slides, *b.Slide)
		}
		p := ooxml.NewPptx(ooxml.Theme{}, d.Title)
		p.BaseDir = st.Root
		warns, err := p.BuildSlides(spec)
		if err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		// Una imatge que falta no ha d'impedir la baixada.
		for _, warn := range warns {
			log.Printf("docs: %s: %s", d.ID, warn)
		}
		pkg = p.Pkg
		ctype = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		x := ooxml.NewDocx(ooxml.DocxOptions{Title: d.Title, PageNumbers: true})
		x.AppendMarkdown(d.Markdown())
		pkg = x.Pkg
	}
	if pkg != nil {
		data, err = pkg.Bytes()
	}
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	name := d.Title + "." + format
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+senseEspais(name)+"\"; filename*=UTF-8''"+url.PathEscape(name))
	_, _ = w.Write(data)
}

// sheetXlsx és el llibre d'Excel d'un full de càlcul, amb les fórmules.
func sheetXlsx(d gdocs.Doc) ([]byte, error) {
	f, err := gdocs.Workbook(d)
	if err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return buf.Bytes(), f.Close()
}

// docStore obre els documents del projecte de la sessió, o amb ?scope=mine
// els de la carpeta de documents dels vents de qui crida; si no pot, ja ha
// respost.
func (s *Server) docStore(w http.ResponseWriter, r *http.Request) (*gdocs.Store, bool) {
	st, err := gdocs.Open(s.sessionCwd())
	if r.URL.Query().Get("scope") == "mine" {
		st, err = gdocs.OpenAt(s.windDocsDir())
	}
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return nil, false
	}
	return st, true
}

// sessionDoc llegeix el document {id} del projecte de la sessió; si no pot,
// ja ha respost (404 si és d'un altre projecte o no existeix).
func (s *Server) sessionDoc(w http.ResponseWriter, r *http.Request) (*gdocs.Store, gdocs.Doc, bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return nil, gdocs.Doc{}, false
	}
	st, ok := s.docStore(w, r)
	if !ok {
		return nil, gdocs.Doc{}, false
	}
	d, err := st.Get(r.PathValue("id"))
	if errors.Is(err, gdocs.ErrNotFound) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return nil, d, false
	}
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return nil, d, false
	}
	return st, d, true
}

// handleDocs llista els documents del projecte de la sessió, l'últim
// canviat primer. Amb POST la persona en crea un de text buit, sense
// l'agent («Document nou» de Gregal Docs).
//
//	GET /api/docs
//	POST /api/docs {title, kind:"doc"}
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleDocCreate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
		return
	}
	st, ok := s.docStore(w, r)
	if !ok {
		return
	}
	docs, err := st.List()
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		items = append(items, map[string]any{"id": d.ID, "title": d.Title, "kind": d.Kind, "version": d.Version, "updated": d.Updated})
	}
	writeJSON(w, map[string]any{"items": items})
}

// handleDocCreate fa el document nou de la persona. Neix sense blocs: el
// primer que hi escriu entra per les rutes de blocs, amb autor user.
func (s *Server) handleDocCreate(w http.ResponseWriter, r *http.Request) {
	var q struct{ Title, Kind string }
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if q.Kind != gdocs.KindDoc {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "kind: only \"doc\" can be created here"})
		return
	}
	if strings.TrimSpace(q.Title) == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	st, ok := s.docStore(w, r)
	if !ok {
		return
	}
	d, err := st.Create(q.Title, "", gdocs.AuthorUser)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, d)
}

// handleDoc torna un document sencer.
//
//	GET /api/docs/{id}
func (s *Server) handleDoc(w http.ResponseWriter, r *http.Request) {
	if _, d, ok := s.sessionDoc(w, r); ok {
		writeJSON(w, d)
	}
}

// handleDocStream envia un event `block` per cada canvi del document des
// que s'obre. Les capçaleres surten de seguida, ja subscrit, perquè el
// client pugui llegir el document després sense perdre cap canvi.
//
//	GET /api/docs/{id}/stream
func (s *Server) handleDocStream(w http.ResponseWriter, r *http.Request) {
	st, d, ok := s.sessionDoc(w, r)
	if !ok {
		return
	}
	changes, stop := st.Watch(d.ID)
	defer stop()
	emit, stopHeartbeat := sseWithHeartbeat(w, r.Context(), 15*time.Second)
	defer stopHeartbeat()
	w.WriteHeader(http.StatusOK)
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case c, open := <-changes:
			if !open {
				return
			}
			emit("block", c)
		}
	}
}

// handleDocPage serveix un document a pàgina sencera, la finestra pròpia
// que s'obre des del panell: el mateix panell (docs-panel.js) sense la
// resta de l'app. Sense comptes, un id que no és del projecte de la sessió
// (?session=) és un 404. Amb comptes la navegació no porta el token i no
// se sap de qui és la sessió (ni se'n crea cap): ho diu la pàgina en llegir
// /api/docs/{id}.
//
//	GET /app/doc/{id}?session=
func (h *Hub) handleDocPage(w http.ResponseWriter, r *http.Request) {
	if d := h.def; d.Token == "" && (d.users == nil || !d.users.Actiu()) {
		if _, _, ok := h.resolve(r).sessionDoc(w, r); !ok {
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, `<!doctype html><html lang="ca"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Gregal</title><style>`+tema.CSSRoot()+
		`html,body{margin:0;height:100%;background:var(--panel);color:var(--ink);font:14px/1.55 system-ui,sans-serif}#workbench{display:flex;height:100%}`+
		`body #docsPanel{position:static;flex:1;width:auto;border:0;box-shadow:none}body #docsPanel header{display:none}</style>`+
		`<script>(()=>{let t='';try{t=(JSON.parse(localStorage.getItem('gregal_brand_cache')||'null')||{}).theme||localStorage.getItem('gregal_tema')||''}catch(e){}`+
		`if(!t||t==='sistema')t=matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';document.documentElement.setAttribute('data-theme',t)})()</script>`+
		`</head><body><div id="workbench"></div><script src="/app/md.js"></script><script src="/app/doc-view.js"></script>`+
		`<script type="module" src="/app/doc-page.js"></script></body></html>`)
}

// handleDocBlocks insereix blocs que escriu la persona: md es parteix com
// fa l'eina, i la resposta porta els blocs nous en ordre.
//
//	POST /api/docs/{id}/blocks {after, md}
func (s *Server) handleDocBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var q struct{ After, MD string }
	if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.MD) == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "md required"})
		return
	}
	st, ok := s.docStore(w, r)
	if !ok {
		return
	}
	d, err := st.Insert(r.PathValue("id"), q.After, q.MD, gdocs.AuthorUser)
	if docWriteErr(w, err) {
		return
	}
	nous := []gdocs.Block{}
	for _, b := range d.Blocks {
		if b.Since == d.Version {
			nous = append(nous, b)
		}
	}
	writeJSON(w, map[string]any{"version": d.Version, "blocks": nous})
}

// handleDocBlock canvia o esborra un bloc com a persona. base_version és
// la versió del bloc que la persona tenia: si algú l'ha canviat després,
// 409 amb el bloc d'ara, i no es toca res.
//
//	PUT /api/docs/{id}/blocks/{block} {md, base_version}
//	PUT /api/docs/{id}/blocks/{block} {cells, base_version} (un full)
//	DELETE /api/docs/{id}/blocks/{block}?base_version=N
func (s *Server) handleDocBlock(w http.ResponseWriter, r *http.Request) {
	var q struct {
		MD    string            `json:"md"`
		Cells map[string]string `json:"cells"`
		Base  int               `json:"base_version"`
	}
	switch r.Method {
	case http.MethodPut:
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
	case http.MethodDelete:
		var err error
		if q.Base, err = strconv.Atoi(r.URL.Query().Get("base_version")); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "base_version required"})
			return
		}
	default:
		http.Error(w, "PUT or DELETE", http.StatusMethodNotAllowed)
		return
	}
	st, ok := s.docStore(w, r)
	if !ok {
		return
	}
	id, block := r.PathValue("id"), r.PathValue("block")
	if r.Method == http.MethodDelete {
		d, err := st.Delete(id, block, gdocs.AuthorUser, q.Base)
		if !docWriteErr(w, err) {
			writeJSON(w, map[string]any{"version": d.Version})
		}
		return
	}
	var d gdocs.Doc
	var err error
	if q.Cells != nil {
		d, err = st.SetCells(id, block, q.Cells, gdocs.AuthorUser, q.Base)
	} else {
		d, err = st.Replace(id, block, q.MD, gdocs.AuthorUser, q.Base)
	}
	if docWriteErr(w, err) {
		return
	}
	for _, b := range d.Blocks {
		if b.ID == block {
			writeJSON(w, map[string]any{"version": d.Version, "block": b})
		}
	}
}

// docWriteErr respon l'error d'una escriptura, si n'hi ha.
func docWriteErr(w http.ResponseWriter, err error) bool {
	var c *gdocs.ConflictError
	switch {
	case err == nil:
		return false
	case errors.As(err, &c):
		writeJSONStatus(w, http.StatusConflict, map[string]any{"error": err.Error(), "block": c.Block})
	case errors.Is(err, gdocs.ErrNotFound), errors.Is(err, gdocs.ErrBlockNotFound):
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return true
}

// Les sortides de l'eina doc que han escrit: «Created document <id>» i
// «Document <id> · …» (agent/doc_tool.go).
var docToolOut = regexp.MustCompile(`^(?:Created document|Document) ([a-z0-9]+) `)

// emitDocChanged avisa la sessió que l'agent ha creat o canviat un
// document de root, perquè la web l'obri al panell lateral. scope, si no
// és buit, és el que el client ha de passar a les rutes (mine: els dels
// vents).
func (s *Server) emitDocChanged(emit func(string, any), root, scope, argsJSON, out string) {
	var a struct{ Action string }
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create", "insert", "replace", "set_cells", "delete":
	default:
		return
	}
	m := docToolOut.FindStringSubmatch(out)
	if m == nil {
		return
	}
	open := gdocs.Open
	if scope == "mine" {
		open = gdocs.OpenAt
	}
	st, err := open(root)
	if err != nil {
		return
	}
	if d, err := st.Get(m[1]); err == nil {
		ev := map[string]string{"id": d.ID, "title": d.Title}
		if scope != "" {
			ev["scope"] = scope
		}
		emit("doc_changed", ev)
	}
}
