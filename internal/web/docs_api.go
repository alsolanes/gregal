package web

import (
	"errors"
	"net/http"
	"net/url"

	"gregal/internal/gdocs"
	"gregal/internal/ooxml"
)

// handleDocExport baixa un document viu del projecte de la sessió com a
// fitxer d'Office, fet dels blocs amb el tema de la marca activa.
//
//	GET /api/docs/{id}/export?format=docx
func (s *Server) handleDocExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if f := r.URL.Query().Get("format"); f != "" && f != "docx" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "format: only docx"})
		return
	}
	st, err := gdocs.Open(s.sessionCwd())
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	d, err := st.Get(r.PathValue("id"))
	if errors.Is(err, gdocs.ErrNotFound) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Sense tema, NewDocx fa servir el per defecte, que és la marca activa.
	x := ooxml.NewDocx(ooxml.DocxOptions{Title: d.Title, PageNumbers: true})
	x.AppendMarkdown(d.Markdown())
	data, err := x.Pkg.Bytes()
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	name := d.Title + ".docx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+senseEspais(name)+"\"; filename*=UTF-8''"+url.PathEscape(name))
	_, _ = w.Write(data)
}
