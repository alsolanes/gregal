package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"gregal/internal/tema"
)

// El markdown de les respostes, renderitzat al servidor.
//
// El client en tenia un de propi (app/md.js, 157 línies) que cobria el que
// els models solen escriure, però no tot: en una conversa real, una taula
// va sortir com un paràgraf massís en negreta de quaranta línies, i els
// blocs de codi mai no han tingut color. Mantenir un segon renderitzador
// de markdown a mà és feina que no acaba mai.
//
// Aquí es fa amb goldmark (el mateix que hi ha darrere de glamour, que és
// qui pinta el markdown del TUI: els dos fronts entenen la mateixa
// gramàtica) i chroma per al codi, amb la paleta d'internal/tema. El
// resultat passa per bluemonday: el model no pot injectar HTML ni encara
// que ho intenti.
//
// app/md.js es queda per al text que arriba en directe, que ha de pintar-se
// a cada fragment i no pot fer un viatge al servidor per cada un.

// politica és el que es deixa passar de l'HTML generat. Es construeix una
// vegada: bluemonday compila expressions regulars en crear-la.
var (
	politicaUn sync.Once
	politica   *bluemonday.Policy
)

func politicaHTML() *bluemonday.Policy {
	politicaUn.Do(func() {
		p := bluemonday.UGCPolicy()
		// Taules (GFM) i el marcatge del codi.
		p.AllowElements("table", "thead", "tbody", "tr", "th", "td", "pre", "code", "span", "del", "input")
		p.AllowAttrs("align").OnElements("th", "td")
		// Chroma escriu els colors com a estil en línia: el CSS del web no
		// té full de sintaxi, i així el mateix HTML val per al tema que hi
		// hagi. Només es deixa passar `color` i `font-weight/style`.
		p.AllowAttrs("style").Matching(estilSegur).OnElements("span")
		p.AllowAttrs("class").Matching(classeSegura).OnElements("pre", "code", "span", "div")
		// Caselles de les llistes de tasques, sempre desactivades.
		p.AllowAttrs("type", "checked", "disabled").OnElements("input")
		p.RequireNoFollowOnLinks(true)
		p.AddTargetBlankToFullyQualifiedLinks(true)
		politica = p
	})
	return politica
}

// markdown és el renderitzador, un per tema (chroma hi va a dins).
var (
	mdMu  sync.Mutex
	mdPer = map[string]goldmark.Markdown{}
)

func renderitzador(t tema.Tema) goldmark.Markdown {
	mdMu.Lock()
	defer mdMu.Unlock()
	if md, ok := mdPer[t.Nom]; ok {
		return md
	}
	md := goldmark.New(
		// GFM: taules, ratllat, autoenllaç i llistes de tasques. És el que
		// els models escriuen de debò.
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(&codiHTML{t: t}, 1)),
		),
	)
	mdPer[t.Nom] = md
	return md
}

// Markdown converteix el text del model en HTML segur.
func Markdown(text string, t tema.Tema) string {
	var b bytes.Buffer
	if err := renderitzador(t).Convert([]byte(text), &b); err != nil {
		// Sense markdown vàlid val més el text pla que una pàgina buida.
		return "<p>" + bluemonday.StrictPolicy().Sanitize(text) + "</p>"
	}
	return politicaHTML().Sanitize(b.String())
}

// codiHTML pinta els blocs de codi amb chroma en comptes de deixar-los en
// text pla. És l'única part del renderitzat que canviem.
type codiHTML struct{ t tema.Tema }

func (c *codiHTML) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, c.bloc)
}

func (c *codiHTML) bloc(w util.BufWriter, font []byte, n ast.Node, entra bool) (ast.WalkStatus, error) {
	if !entra {
		return ast.WalkContinue, nil
	}
	b := n.(*ast.FencedCodeBlock)
	llengua := string(b.Language(font))
	var codi strings.Builder
	for i := 0; i < b.Lines().Len(); i++ {
		l := b.Lines().At(i)
		codi.Write(l.Value(font))
	}
	text := codi.String()

	classe := "codeblock"
	if llengua != "" {
		classe += " lang-" + netejaClasse(llengua)
	}
	w.WriteString(`<pre class="` + classe + `"><code>`)
	if pintat, ok := tema.PintaCodiHTML(c.t, llengua, text); ok {
		w.WriteString(pintat)
	} else {
		// Sense lexer fiable, text escapat: millor codi sense color que
		// codi mal pintat.
		w.WriteString(escapaHTML(text))
	}
	w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

func escapaHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// netejaClasse deixa només lletres, xifres, guió i guió baix: la llengua
// la posa el model i acaba en un atribut class.
func netejaClasse(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

// estilSegur i classeSegura acoten el que bluemonday deixa passar als
// atributs que genera chroma: colors i pesos, i noms de classe nostres.
var (
	estilSegur   = regexp.MustCompile(`^(?:(?:color|background-color):\s*#[0-9a-fA-F]{3,8};?|font-weight:\s*(?:bold|normal);?|font-style:\s*(?:italic|normal);?|\s)+$`)
	classeSegura = regexp.MustCompile(`^[a-z0-9 _-]{1,64}$`)
)

// handleMarkdown converteix el text d'una resposta en HTML segur. El
// client el crida quan el torn ja ha acabat i té el text sencer; mentre
// arriba, pinta amb app/md.js, que no ha de fer cap viatge.
func (s *Server) handleMarkdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "cal POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Text string `json:"text"`
		Tema string `json:"tema"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "cos il·legible", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"html": Markdown(req.Text, tema.Per(req.Tema))})
}
