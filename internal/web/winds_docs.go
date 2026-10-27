package web

// Documents dels vents: al xat, un vent pot crear PDF i documents d'Office
// (pdf_create, office_create) i editar-los (pdf_edit, office_edit), cosa
// que el xat de només lectura denegava i el vent responia «no tinc les
// eines activades». Escriu només a la seva carpeta de documents (per usuari)
// o sobre els documents que l'usuari ha adjuntat; la resposta du l'enllaç
// de baixada, que l'app obre amb la mateixa autenticació que l'API.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/tools"
)

type windDocsKey struct{}

// withWindDocs marca el context d'un torn de vent: les eines de documents
// hi escriuen a dir.
func withWindDocs(ctx context.Context, dir string) context.Context {
	if dir == "" {
		return ctx
	}
	return context.WithValue(ctx, windDocsKey{}, dir)
}

func windDocsOf(ctx context.Context) string {
	d, _ := ctx.Value(windDocsKey{}).(string)
	return d
}

// windDocsDir és la carpeta de documents dels vents de l'usuari.
func (s *Server) windDocsDir() string {
	return filepath.Join(s.windStateDir(), "wind-documents")
}

// windDocsCtx prepara el context d'un torn de vent amb la carpeta creada.
func (s *Server) windDocsCtx(ctx context.Context) context.Context {
	dir := s.windDocsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ctx
	}
	return withWindDocs(ctx, dir)
}

// windDocTool són les eines que escriuen documents.
func windDocTool(name string) bool {
	switch name {
	case "pdf_create", "pdf_edit", "office_create", "office_edit":
		return true
	}
	return false
}

// windDocTargets són els fitxers que una crida escriurà, ja resolts
// contra la carpeta del vent.
func windDocTargets(dir, name, argsJSON string) []string {
	var a struct {
		Path string `json:"path"`
		Op   string `json:"op"`
		Out  string `json:"out"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	abs := func(p string) string {
		p = strings.TrimSpace(p)
		if p == "" {
			return ""
		}
		if !tools.IsRooted(p) {
			p = filepath.Join(dir, p)
		}
		return filepath.Clean(p)
	}
	if name == "pdf_edit" {
		var out []string
		for _, t := range tools.PdfEditTargets(tools.PdfEditOpts{Path: abs(a.Path), Op: a.Op, Out: abs(a.Out)}) {
			out = append(out, t)
		}
		return out
	}
	return []string{abs(a.Path)}
}

// windDocAllowed: dins la carpeta del vent, o un document adjuntat per
// l'usuari (pujat a /api/office/upload).
func windDocAllowed(dir, target string) bool {
	if target == "" {
		return false
	}
	if inside(dir, target) {
		return true
	}
	return inside(os.TempDir(), target) && filepath.Dir(filepath.Clean(target)) == filepath.Clean(os.TempDir()) &&
		strings.HasPrefix(filepath.Base(target), "gregal-office-")
}

func inside(dir, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// windDocDecide és el permís d'una eina de documents en un torn de vent.
func windDocDecide(dir, name, argsJSON string) (string, string) {
	targets := windDocTargets(dir, name, argsJSON)
	if len(targets) == 0 {
		return "deny", "document: falta la ruta"
	}
	for _, t := range targets {
		if !windDocAllowed(dir, t) {
			return "deny", "els vents només desen documents a la seva carpeta: fes servir un nom de fitxer sol, com informe.pdf"
		}
	}
	return "allow", "document del vent"
}

// windDocsBrief és la consigna dels documents per al prompt del vent.
const windDocsBrief = "\n\nDOCUMENTS: when the user asks for a document (a PDF, a report, a letter, a CV, a Word, Excel or PowerPoint file), create it yourself: pdf_create (a PDF from markdown) or office_create (.docx/.xlsx/.pptx), and change it with pdf_edit or office_edit. Use a plain file name such as informe-vendes.pdf: it is saved in your documents folder and the app adds a download link to your reply by itself, so never write paths or links to it. Documents the user attached (an @path in the message) can be read with pdf_read or office_read and edited in place. Never say you cannot create files."

// windDocLinks són els enllaços de baixada dels documents creats o
// editats des de since: els de la carpeta del vent i els adjunts editats.
func (s *Server) windDocLinks(since time.Time, reply string) string {
	type doc struct {
		name, link string
		at         time.Time
	}
	var docs []doc
	dir := s.windDocsDir()
	since = since.Add(-time.Second)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			fi, err := e.Info()
			if err != nil || fi.ModTime().Before(since) {
				continue
			}
			docs = append(docs, doc{e.Name(), "/api/winds/doc?name=" + url.QueryEscape(e.Name()), fi.ModTime()})
		}
	}
	officeStore.Lock()
	for id, d := range officeStore.docs {
		if d.owner != "" && d.owner != s.usuari() {
			continue
		}
		if fi, err := os.Stat(d.path); err == nil && !fi.ModTime().Before(since) && fi.ModTime().After(d.at.Add(time.Second)) {
			docs = append(docs, doc{d.name, "/api/office/download?id=" + id, fi.ModTime()})
		}
	}
	officeStore.Unlock()
	sort.Slice(docs, func(i, j int) bool { return docs[i].at.Before(docs[j].at) })
	var b strings.Builder
	for _, d := range docs {
		if strings.Contains(reply, d.link) {
			continue
		}
		fmt.Fprintf(&b, "\n📄 [%s](%s)", d.name, d.link)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n" + b.String()
}

// handleWindDoc baixa un document de la carpeta dels vents.
//
//	GET /api/winds/doc?name=informe.pdf
func (s *Server) handleWindDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\`) {
		http.Error(w, "nom de document invàlid", http.StatusBadRequest)
		return
	}
	p := filepath.Join(s.windDocsDir(), name)
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		http.Error(w, "document inexistent", http.StatusNotFound)
		return
	}
	ct := "application/octet-stream"
	if tools.IsPDF(name) {
		ct = "application/pdf"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+senseEspais(name)+"\"; filename*=UTF-8''"+url.PathEscape(name))
	http.ServeFile(w, r, p)
}
