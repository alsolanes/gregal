package web

// Office via API (per a l'app Android i el web): el fitxer viu al
// TELÈFON/navegador, no al servidor. Flux: upload (base64) → read/edit
// per id → download. Els fitxers pugen a tmp, caduquen en 2h i el límit
// és 25MB. L'edició passa pel journal: el rewind del servidor també
// restaura aquests fitxers.

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/tools"
)

type officeDoc struct {
	name string
	kind string
	path string
	at   time.Time
	// owner és qui l'ha pujat: un document d'un usuari no el pot llegir
	// ni descarregar un altre ni que encerti l'id.
	owner string
}

var officeStore = struct {
	sync.Mutex
	docs map[string]*officeDoc
}{docs: map[string]*officeDoc{}}

func officeSweep() {
	officeStore.Lock()
	defer officeStore.Unlock()
	for id, d := range officeStore.docs {
		if time.Since(d.at) > 2*time.Hour {
			os.Remove(d.path)
			delete(officeStore.docs, id)
		}
	}
}

func officeGet(id string) (*officeDoc, bool) { return officeGetFor(id, "") }

// officeGetFor només torna el document si és de qui el demana. `owner`
// buit vol dir servidor local sense usuaris: tot és de tothom, com abans.
func officeGetFor(id, owner string) (*officeDoc, bool) {
	officeSweep()
	officeStore.Lock()
	defer officeStore.Unlock()
	d, ok := officeStore.docs[id]
	if !ok {
		return nil, false
	}
	if d.owner != "" && owner != "" && d.owner != owner {
		return nil, false
	}
	return d, true
}

func (s *Server) handleOfficeUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
		Data string `json:"data"` // base64 del fitxer sencer
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	kind := tools.OfficeKind(req.Name)
	if kind != "xlsx" && kind != "docx" && kind != "pptx" {
		http.Error(w, "només .docx/.xlsx/.pptx (i no formats antics .doc/.xls/.ppt)", http.StatusBadRequest)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		http.Error(w, "base64 invàlid", http.StatusBadRequest)
		return
	}
	if len(raw) > 25<<20 {
		http.Error(w, "fitxer massa gran (25 MB)", http.StatusBadRequest)
		return
	}
	var idb [8]byte
	rand.Read(idb[:])
	id := hex.EncodeToString(idb[:])
	safe := strings.ReplaceAll(filepath.Base(req.Name), "..", "_")
	// El nom del fitxer al disc no pot dur espais: la ruta es dona a
	// l'agent com a «@ruta» i el lector de mencions talla al primer espai,
	// o sigui que «EWEC EDH Project Plan.xlsx» arribava tallat i el model
	// deia, amb raó, que el fitxer no existia. El nom bonic es conserva a
	// officeDoc.name, que és el que es veu a la pantalla.
	path := filepath.Join(os.TempDir(), "gregal-office-"+id+"-"+senseEspais(safe))
	if err := os.WriteFile(path, raw, 0600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Valida que obre de debò.
	if _, err := tools.OfficeRead(path); err != nil {
		os.Remove(path)
		http.Error(w, "el fitxer no obre: "+err.Error(), http.StatusBadRequest)
		return
	}
	officeStore.Lock()
	officeStore.docs[id] = &officeDoc{name: safe, kind: kind, path: path, at: time.Now(), owner: s.usuari()}
	officeStore.Unlock()
	// path: perquè la UI pugui donar el document a l'agent amb @ruta (les
	// eines office_read/office_edit treballen amb rutes, no amb ids).
	writeJSON(w, map[string]any{"id": id, "kind": kind, "name": safe, "size": len(raw), "path": path})
}

func (s *Server) handleOfficeRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	d, ok := officeGetFor(req.ID, s.usuari())
	if !ok {
		http.Error(w, "document caducat o inexistent (torna'l a pujar)", http.StatusNotFound)
		return
	}
	out, err := tools.OfficeRead(d.path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(out) > 20000 {
		out = out[:20000] + "\n… (retallat)"
	}
	writeJSON(w, map[string]any{"id": req.ID, "kind": d.kind, "name": d.name, "text": out})
}

func (s *Server) handleOfficeEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       string            `json:"id"`
		Op       string            `json:"op"`
		Sheet    string            `json:"sheet"`
		Cell     string            `json:"cell"`
		Value    string            `json:"value"`
		Find     string            `json:"find"`
		Replace  string            `json:"replace"`
		Content  string            `json:"content"`
		Anchor   string            `json:"anchor"`
		Position string            `json:"position"`
		Text     string            `json:"text"`
		Author   string            `json:"author"`
		Track    bool              `json:"track"`
		Slide    int               `json:"slide"`
		To       int               `json:"to"`
		Layout   string            `json:"layout"`
		Spec     string            `json:"spec"`
		Shapes   map[string]string `json:"shapes"`
		Cells    map[string]string `json:"cells"`
		Vars     map[string]string `json:"vars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	d, ok := officeGetFor(req.ID, s.usuari())
	if !ok {
		http.Error(w, "document caducat o inexistent (torna'l a pujar)", http.StatusNotFound)
		return
	}
	if req.Op == "" {
		http.Error(w, "falta op", http.StatusBadRequest)
		return
	}
	// Les mateixes operacions que l'eina office_edit (tools.OfficeEdit les
	// valida segons el tipus de document).
	out, err := tools.OfficeEdit(tools.EditOpts{
		Path: d.path, Op: req.Op, Sheet: req.Sheet, Cell: req.Cell, Value: req.Value,
		Find: req.Find, Replace: req.Replace, Content: req.Content, Anchor: req.Anchor, Position: req.Position,
		Text: req.Text, Author: req.Author, Track: req.Track, Slide: req.Slide, To: req.To, Layout: req.Layout,
		Spec: req.Spec, Shapes: req.Shapes, Cells: req.Cells, Vars: req.Vars,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Refresca el text perquè l'app el mostri al moment.
	text, _ := tools.OfficeRead(d.path)
	if len(text) > 20000 {
		text = text[:20000] + "\n… (retallat)"
	}
	writeJSON(w, map[string]any{"result": out, "text": text})
}

// handleOfficeRender renderitza el document (PNG en data URL) i torna la
// QA automàtica, perquè la UI ensenyi com quedarà de debò.
func (s *Server) handleOfficeRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string `json:"id"`
		Pages string `json:"pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	d, ok := officeGetFor(req.ID, s.usuari())
	if !ok {
		http.Error(w, "document caducat o inexistent (torna'l a pujar)", http.StatusNotFound)
		return
	}
	out, imgs, err := tools.OfficeRender(r.Context(), tools.RenderOpts{Path: d.path, Pages: req.Pages})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"report": out, "images": imgs, "engine": tools.OfficeRenderAvailable()})
}

func (s *Server) handleOfficeOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	d, ok := officeGetFor(req.ID, s.usuari())
	if !ok {
		http.Error(w, "document caducat o inexistent (torna'l a pujar)", http.StatusNotFound)
		return
	}
	// El fitxer viu al servidor: només té sentit obrir-lo si el servidor
	// és local (desktop). La UI amaga el botó quan no ho és.
	out, err := tools.OfficeOpen(d.path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"result": out})
}

func (s *Server) handleOfficeDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	d, ok := officeGetFor(r.URL.Query().Get("id"), s.usuari())
	if !ok {
		http.Error(w, "document caducat o inexistent", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+d.name+"\"")
	http.ServeFile(w, r, d.path)
}

// senseEspais treu de l'anomenada tot el que faria trontollar una ruta
// passada com a «@ruta»: espais i les cometes i comes que tallen la
// menció. La resta (accents, parèntesis) es queda: no molesta ningú.
func senseEspais(nom string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', ',', ';', '\'', '"':
			return '_'
		}
		return r
	}, nom)
}
