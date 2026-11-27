package web

import (
	"encoding/json"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gregal/internal/llm"
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
// resposta encara no esmenta i que aquest torn ha escrit (hist): amb dos
// torns alhora, la web de l'altre vent també és nova i no és d'aquesta resposta.
func (s *Server) windWebCards(since time.Time, reply string, hist []llm.Message) string {
	written := turnWrittenFiles(hist)
	if len(written) == 0 {
		return ""
	}
	since = since.Add(-time.Second)
	var cards []webCard
	seenDir := map[string]bool{}
	for _, root := range s.previewRoots() {
		if len(cards) >= webCardMax {
			break
		}
		for _, w := range previewWebs(root.Dir) {
			mt, err := time.Parse(time.RFC3339, w.Mtime)
			if err != nil || mt.Before(since) || !wroteInDir(written, filepath.ToSlash(filepath.Join(root.Dir, path.Dir(w.Path)))) {
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

// htmlInCommand treu les rutes .html d'una ordre de shell (cat > x.html, cp …).
var htmlInCommand = regexp.MustCompile(`[^\s"'<>|;&]+\.html?\b`)

// turnWrittenFiles són les rutes que han escrit les eines d'un torn: write,
// edit i patch pel camp path, i les .html que esmenta una ordre de shell.
func turnWrittenFiles(hist []llm.Message) []string {
	var out []string
	for _, m := range hist {
		for _, c := range m.ToolCalls {
			switch c.Function.Name {
			case "write", "edit", "patch":
				var a struct{ Path string }
				if json.Unmarshal([]byte(c.Function.Arguments), &a) == nil && a.Path != "" {
					out = append(out, a.Path)
				}
			case "bash", "bash_background":
				var a struct{ Command string }
				if json.Unmarshal([]byte(c.Function.Arguments), &a) == nil {
					out = append(out, htmlInCommand.FindAllString(a.Command, -1)...)
				}
			}
		}
	}
	return out
}

// wroteInDir diu si alguna ruta escrita és dins de la carpeta dir (absoluta,
// amb /). Una ruta relativa compta si la carpeta acaba igual («rius/index.html»
// és de …/projectes/rius); una de sola («index.html»), si el nom de la carpeta
// no hi diu res, no compta: no se sap de quina és.
func wroteInDir(written []string, dir string) bool {
	dir = strings.ToLower(strings.TrimSuffix(dir, "/"))
	for _, p := range written {
		d := strings.ToLower(path.Dir(path.Clean(filepath.ToSlash(p))))
		if d == dir || (d != "." && !filepath.IsAbs(p) && !strings.HasPrefix(d, "/") && strings.HasSuffix(dir, "/"+strings.TrimPrefix(d, "./"))) {
			return true
		}
	}
	return false
}

// webCardTitle: «calculadora-propines» → «Calculadora propines».
func webCardTitle(slug string) string {
	t := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(slug))
	if t == "" {
		return slug
	}
	return strings.ToUpper(t[:1]) + t[1:]
}
