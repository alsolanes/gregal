package web

import (
	"encoding/json"
	"path"
	"strings"
	"time"
)

// La web que un vent acaba de fer, dins del xat.
//
// Abans el vent deia «ja el tens a la vista» i al xat només hi havia la ruta
// del fitxer: per veure-la calia anar a una altra pantalla. Ara, si el torn ha
// creat o canviat una web (un .html a les carpetes que ja es previsualitzen,
// preview.go), la resposta porta una targeta ```gregal-web amb el nom i
// l'adreça /p/<clau>/…: l'escriptori la pinta funcionant en un iframe aïllat i
// l'Android en un WebView, com els artefactes de Claude o el canvas de Grok.

const webCardMax = 2

type webCard struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	URL   string `json:"url"`
}

// windWebCards torna les targetes de les webs tocades des de [since] que la
// resposta encara no esmenta.
func (s *Server) windWebCards(since time.Time, reply string) string {
	since = since.Add(-time.Second)
	var cards []webCard
	seenDir := map[string]bool{}
	for _, root := range s.previewRoots() {
		for _, w := range previewWebs(root.Dir) {
			mt, err := time.Parse(time.RFC3339, w.Mtime)
			if err != nil || mt.Before(since) {
				continue
			}
			rel := root.Slug + "/" + w.Path
			dir := path.Dir(rel)
			// Una web per carpeta: index.html primer (previewWebs ja l'ordena així).
			if seenDir[dir] {
				continue
			}
			seenDir[dir] = true
			u := s.previewBase() + rel
			if strings.Contains(reply, u) || strings.Contains(reply, "```gregal-web") {
				continue
			}
			title := path.Base(dir)
			if title == root.Slug || title == "." {
				title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
			}
			cards = append(cards, webCard{Title: webCardTitle(title), Path: rel, URL: u})
			if len(cards) >= webCardMax {
				break
			}
		}
	}
	var b strings.Builder
	for _, c := range cards {
		raw, _ := json.Marshal(c)
		b.WriteString("\n\n```gregal-web\n")
		b.Write(raw)
		b.WriteString("\n```")
	}
	return b.String()
}

// webCardTitle: «calculadora-propines» → «Calculadora propines».
func webCardTitle(slug string) string {
	t := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(slug))
	if t == "" {
		return slug
	}
	return strings.ToUpper(t[:1]) + t[1:]
}
