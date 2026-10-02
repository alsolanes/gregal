package tools

// Lector web per a agents: d'una pàgina HTML en treu el contingut
// principal (article, documentació, fil) i el torna en markdown compacte,
// sense menús, peus, banners, formularis ni imatges. És el que un model
// necessita llegir i res més: una pàgina de 300 KB de HTML acostuma a
// quedar en 3-8 KB de text útil.
//
// L'algorisme és un readability petit: es puntuen els blocs de text
// (paràgrafs, llistes, cel·les, capçaleres) i la puntuació puja al pare i
// a l'avi; guanya el contenidor amb més text propi i menys densitat
// d'enllaços, amb bonificació o penalització per pistes de class/id
// (article, content, post / nav, sidebar, footer, comment…). Els germans
// del guanyador amb prou puntuació s'hi afegeixen (articles partits en
// diversos div). Sense cap candidat clar, es pren el body sencer.
//
// No depèn de res fora de golang.org/x/net/html.

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// WebDoc és el resultat de llegir una pàgina.
type WebDoc struct {
	Title     string
	Byline    string
	Published string
	Canonical string
	Markdown  string // contingut principal en markdown
	Words     int    // paraules del Markdown
	Whole     bool   // true si no s'ha trobat cap bloc principal i s'ha pres tota la pàgina
}

var (
	reNegatiu = regexp.MustCompile(`(?i)(^|[\s_-])(nav|navbar|menu|sidebar|side-bar|footer|foot|comment|comments|cookie|banner|advert|ads?|promo|share|sharing|social|related|breadcrumb|breadcrumbs|popup|modal|subscribe|newsletter|widget|toolbar|pagination|skip|hidden|masthead|disclaimer|legal|byline-links|sponsor|dropdown|portlet|interlanguage|sitenotice|editsection|catlinks|printfooter|indicators|noprint|toc|jump)([\s_-]|$)`)
	rePositiu = regexp.MustCompile(`(?i)(^|[\s_-])(article|content|main|post|entry|body|text|story|blog|page|markdown|readme|documentation|docs|prose)([\s_-]|$)`)
	reEspais  = regexp.MustCompile(`[ \t\r\f\v\x{00a0}]+`)
	reLinies  = regexp.MustCompile(`\n{3,}`)
)

// ReadableHTML llegeix una pàgina i en treu el contingut principal.
// base resol els enllaços relatius (pot ser nil).
func ReadableHTML(src string, base *url.URL) WebDoc {
	var doc WebDoc
	root, err := html.Parse(strings.NewReader(src))
	if err != nil || root == nil {
		doc.Markdown = htmlToText(src)
		doc.Words = comptaParaules(doc.Markdown)
		doc.Whole = true
		return doc
	}
	doc.Title, doc.Byline, doc.Published, doc.Canonical = metadades(root)
	body := troba(root, atom.Body)
	if body == nil {
		body = root
	}
	podaSoroll(body)
	cand := candidatPrincipal(body)
	whole := false
	if cand == nil {
		cand = body
		whole = true
	}
	r := &mdRender{base: base}
	r.node(cand, 0)
	if !whole {
		// Germans amb prou pes (articles partits en diversos div).
		for _, g := range germansAmbPes(cand) {
			r.node(g, 0)
		}
	}
	md := netejaMD(r.b.String())
	// Si el bloc principal és massa magre respecte la pàgina, val més la
	// pàgina sencera que un tros: passa amb índexs, llistes de resultats
	// i pàgines curtes.
	if !whole {
		tot := &mdRender{base: base}
		tot.node(body, 0)
		totMD := netejaMD(tot.b.String())
		if comptaParaules(md) < 60 && comptaParaules(totMD) > 2*comptaParaules(md) {
			md = totMD
			whole = true
		}
	}
	doc.Markdown = md
	doc.Words = comptaParaules(md)
	doc.Whole = whole
	if doc.Title == "" {
		// Primera capçalera del contingut com a títol de recanvi.
		for _, ln := range strings.Split(md, "\n") {
			if strings.HasPrefix(ln, "# ") {
				doc.Title = strings.TrimSpace(ln[2:])
				break
			}
		}
	}
	return doc
}

// ---- metadades ----

func metadades(root *html.Node) (title, byline, published, canonical string) {
	var ogTitle string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if title == "" {
					title = collapseWS(textDe(n))
				}
			case atom.Meta:
				name := strings.ToLower(attr(n, "name"))
				prop := strings.ToLower(attr(n, "property"))
				content := strings.TrimSpace(attr(n, "content"))
				if content == "" {
					break
				}
				switch {
				case prop == "og:title" || name == "twitter:title":
					if ogTitle == "" {
						ogTitle = content
					}
				case name == "author" || prop == "article:author" || name == "dc.creator" || name == "parsely-author":
					if byline == "" && len(content) < 120 && !strings.HasPrefix(content, "http") {
						byline = content
					}
				case prop == "article:published_time" || name == "date" || name == "pubdate" || name == "publishdate" ||
					name == "dc.date" || name == "dc.date.issued" || name == "article:published_time" || name == "parsely-pub-date" ||
					prop == "og:updated_time" || name == "last-modified":
					if published == "" {
						published = retallaData(content)
					}
				}
			case atom.Link:
				if strings.EqualFold(attr(n, "rel"), "canonical") && canonical == "" {
					canonical = strings.TrimSpace(attr(n, "href"))
				}
			case atom.Time:
				if published == "" {
					if dt := strings.TrimSpace(attr(n, "datetime")); dt != "" {
						published = retallaData(dt)
					}
				}
			case atom.Body:
				// El body es recorre a part (només ens cal <time> d'allà);
				// no cal baixar per tot.
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if ogTitle != "" && (title == "" || len(ogTitle) < len(title)) {
		// El <title> sol dur " | Nom del lloc"; l'og:title és el net.
		title = ogTitle
	}
	title = netejaTitol(title)
	return
}

// retallaData deixa la data en YYYY-MM-DD quan ve en ISO.
func retallaData(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return truncateInline(s, 40)
}

// netejaTitol treu el sufix de lloc ("Títol | Diari", "Títol - Docs").
func netejaTitol(t string) string {
	t = collapseWS(t)
	for _, sep := range []string{" | ", " — ", " – ", " :: ", " · "} {
		if i := strings.LastIndex(t, sep); i > 0 && len(t)-i < 40 && i > 10 {
			t = strings.TrimSpace(t[:i])
			break
		}
	}
	return t
}

// ---- poda ----

// podaSoroll elimina el que mai és contingut: scripts, estils, formularis,
// navegació, peus, laterals, ocults i blocs amb class/id de soroll.
func podaSoroll(n *html.Node) {
	var trash []*html.Node
	var walk func(n *html.Node, dinsArticle bool)
	walk = func(n *html.Node, dinsArticle bool) {
		if n.Type == html.CommentNode {
			trash = append(trash, n)
			return
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Noscript, atom.Template, atom.Svg, atom.Iframe,
				atom.Form, atom.Button, atom.Input, atom.Select, atom.Textarea, atom.Option,
				atom.Nav, atom.Aside, atom.Footer, atom.Canvas, atom.Object, atom.Embed,
				atom.Video, atom.Audio, atom.Map, atom.Dialog, atom.Menu:
				trash = append(trash, n)
				return
			case atom.Header:
				if !dinsArticle {
					trash = append(trash, n)
					return
				}
			case atom.Article, atom.Main:
				dinsArticle = true
			}
			role := strings.ToLower(attr(n, "role"))
			switch role {
			case "navigation", "banner", "contentinfo", "complementary", "dialog", "search", "menu", "menubar", "toolbar", "alert", "alertdialog":
				trash = append(trash, n)
				return
			}
			if hasAttr(n, "hidden") || strings.EqualFold(attr(n, "aria-hidden"), "true") {
				trash = append(trash, n)
				return
			}
			if st := strings.ToLower(attr(n, "style")); strings.Contains(st, "display:none") || strings.Contains(st, "display: none") || strings.Contains(st, "visibility:hidden") {
				trash = append(trash, n)
				return
			}
			pista := attr(n, "class") + " " + attr(n, "id")
			if n.DataAtom != atom.Body && n.DataAtom != atom.Article && n.DataAtom != atom.Main &&
				reNegatiu.MatchString(pista) && !rePositiu.MatchString(pista) {
				trash = append(trash, n)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, dinsArticle)
		}
	}
	walk(n, false)
	for _, t := range trash {
		if t.Parent != nil {
			t.Parent.RemoveChild(t)
		}
	}
}

// ---- candidat principal ----

type puntuacio struct {
	m map[*html.Node]float64
}

func candidatPrincipal(body *html.Node) *html.Node {
	// Drecera: un sol <article> o <main> amb text de debò.
	if m := troba(body, atom.Main); m != nil && comptaParaules(textDe(m)) >= 80 {
		if a := troba(m, atom.Article); a != nil && float64(comptaParaules(textDe(a))) >= 0.6*float64(comptaParaules(textDe(m))) {
			return a
		}
		return m
	}
	if arts := trobaTots(body, atom.Article); len(arts) == 1 && comptaParaules(textDe(arts[0])) >= 80 {
		return arts[0]
	}
	p := &puntuacio{m: map[*html.Node]float64{}}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.Pre, atom.Td, atom.Blockquote, atom.Li, atom.Dd,
				atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Code:
				txt := collapseWS(textDe(n))
				if len(txt) >= 25 {
					s := 1 + float64(strings.Count(txt, ",")+strings.Count(txt, ".")) + minF(float64(len(txt))/100, 3)
					if par := n.Parent; par != nil {
						p.m[par] += s
						if avi := par.Parent; avi != nil {
							p.m[avi] += s / 2
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(body)
	var millor *html.Node
	var millorScore float64
	for n, s := range p.m {
		if n.Type != html.ElementNode {
			continue
		}
		s += pistaClass(n)
		s *= 1 - densitatEnllacos(n)
		if s > millorScore {
			millor, millorScore = n, s
		}
	}
	if millor == nil || millorScore < 10 {
		return nil
	}
	// Si el pare té gairebé la mateixa puntuació (el text és tot dins un
	// wrapper), puja: així no es perden capçaleres i paràgrafs germans.
	for millor.Parent != nil && millor.Parent != body {
		ps := p.m[millor.Parent]
		ps += pistaClass(millor.Parent)
		ps *= 1 - densitatEnllacos(millor.Parent)
		if ps >= 0.9*millorScore && comptaParaules(textDe(millor.Parent)) <= int(1.6*float64(comptaParaules(textDe(millor))))+40 {
			millor, millorScore = millor.Parent, ps
			continue
		}
		break
	}
	return millor
}

// germansAmbPes retorna els germans del candidat que semblen part del
// mateix article (paràgrafs seguits, no barres laterals).
func germansAmbPes(cand *html.Node) []*html.Node {
	if cand.Parent == nil {
		return nil
	}
	candWords := comptaParaules(textDe(cand))
	var out []*html.Node
	for s := cand.NextSibling; s != nil; s = s.NextSibling {
		if s.Type != html.ElementNode {
			continue
		}
		w := comptaParaules(textDe(s))
		if w < 40 || densitatEnllacos(s) > 0.3 {
			continue
		}
		if w > 3*candWords {
			continue
		}
		if s.DataAtom == atom.P || s.DataAtom == atom.Div || s.DataAtom == atom.Section {
			out = append(out, s)
		}
	}
	return out
}

func pistaClass(n *html.Node) float64 {
	pista := attr(n, "class") + " " + attr(n, "id")
	var s float64
	switch n.DataAtom {
	case atom.Article, atom.Main:
		s += 30
	case atom.Section:
		s += 5
	case atom.Td, atom.Li, atom.Ul, atom.Ol, atom.Table:
		s -= 5
	}
	if rePositiu.MatchString(pista) {
		s += 25
	}
	if reNegatiu.MatchString(pista) {
		s -= 25
	}
	return s
}

func densitatEnllacos(n *html.Node) float64 {
	tot := len(collapseWS(textDe(n)))
	if tot == 0 {
		return 1
	}
	var link int
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			link += len(collapseWS(textDe(n)))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return float64(link) / float64(tot)
}

// ---- render a markdown ----

type mdRender struct {
	b    strings.Builder
	base *url.URL
	// nEnllacos limita el nombre d'enllaços renderitzats: passats els 60,
	// el text dels enllaços surt pelat (llistes de recursos, índexs).
	nEnllacos int
}

func (r *mdRender) node(n *html.Node, depth int) {
	switch n.Type {
	case html.TextNode:
		r.text(n.Data)
		return
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.node(c, depth)
		}
		return
	case html.ElementNode:
	default:
		return
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Template, atom.Svg, atom.Iframe, atom.Img, atom.Picture, atom.Source,
		atom.Form, atom.Button, atom.Input, atom.Select, atom.Textarea, atom.Nav, atom.Aside, atom.Footer, atom.Video, atom.Audio, atom.Canvas:
		return
	case atom.Head:
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		lvl := int(n.Data[1] - '0')
		txt := collapseWS(r.inline(n))
		if txt != "" {
			r.block("\n\n" + strings.Repeat("#", lvl) + " " + txt + "\n\n")
		}
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main, atom.Body, atom.Html, atom.Figure, atom.Figcaption,
		atom.Header, atom.Address, atom.Details, atom.Summary, atom.Dl, atom.Dt, atom.Dd, atom.Center:
		r.blockStart()
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.node(c, depth)
		}
		r.blockEnd()
	case atom.Br:
		r.b.WriteString("\n")
	case atom.Hr:
		r.block("\n\n---\n\n")
	case atom.Pre:
		code := textDe(n)
		code = strings.Trim(code, "\n")
		if strings.TrimSpace(code) != "" {
			lang := llenguatgeCodi(n)
			r.block("\n\n```" + lang + "\n" + code + "\n```\n\n")
		}
	case atom.Blockquote:
		sub := &mdRender{base: r.base, nEnllacos: r.nEnllacos}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sub.node(c, depth)
		}
		r.nEnllacos = sub.nEnllacos
		txt := netejaMD(sub.b.String())
		if txt != "" {
			var q strings.Builder
			for _, ln := range strings.Split(txt, "\n") {
				q.WriteString("> " + ln + "\n")
			}
			r.block("\n\n" + q.String() + "\n")
		}
	case atom.Ul, atom.Ol:
		r.llista(n, depth)
	case atom.Li:
		// Un <li> fora de llista: tracta'l com a paràgraf.
		r.blockStart()
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.node(c, depth)
		}
		r.blockEnd()
	case atom.Table:
		r.taula(n)
	case atom.Tr, atom.Td, atom.Th, atom.Thead, atom.Tbody, atom.Tfoot, atom.Caption:
		// Taula sense <table> ben formada: text pla.
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.node(c, depth)
		}
		r.b.WriteString(" ")
	default:
		r.inlineWalk(n, &r.b)
	}
}

// inline renderitza els fills d'un node en línia (enllaços, negreta,
// codi) i torna el text.
func (r *mdRender) inline(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.inlineWalk(c, &b)
	}
	return b.String()
}

// inlineWalk renderitza el node n (ell mateix, no només els fills) en
// línia dins de b.
func (r *mdRender) inlineWalk(n *html.Node, b *strings.Builder) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
		return
	case html.ElementNode:
	default:
		return
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Svg, atom.Img, atom.Picture, atom.Button, atom.Input, atom.Select, atom.Iframe, atom.Template:
		return
	case atom.Br:
		b.WriteString("\n")
		return
	case atom.A:
		txt := collapseWS(textDe(n))
		href := r.abs(attr(n, "href"))
		if txt == "" {
			return
		}
		if href == "" || r.nEnllacos >= 60 || len(txt) > 90 || strings.EqualFold(txt, href) || strings.HasPrefix(href, "javascript:") {
			b.WriteString(txt)
			return
		}
		r.nEnllacos++
		b.WriteString("[" + txt + "](" + href + ")")
		return
	case atom.Strong, atom.B:
		txt := strings.TrimSpace(r.inline(n))
		if txt != "" {
			b.WriteString(" **" + txt + "** ")
		}
		return
	case atom.Em, atom.I:
		txt := strings.TrimSpace(r.inline(n))
		if txt != "" {
			b.WriteString(" *" + txt + "* ")
		}
		return
	case atom.Code, atom.Kbd, atom.Samp, atom.Tt:
		txt := strings.TrimSpace(textDe(n))
		if txt != "" && !strings.Contains(txt, "\n") {
			b.WriteString("`" + txt + "`")
		} else {
			b.WriteString(txt)
		}
		return
	case atom.Sup:
		b.WriteString(strings.TrimSpace(r.inline(n)))
		return
	case atom.P, atom.Div, atom.Li, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Tr, atom.Pre, atom.Blockquote, atom.Section:
		// Bloc dins d'un inline (HTML mal format): salt de línia.
		b.WriteString("\n")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.inlineWalk(c, b)
		}
		b.WriteString("\n")
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.inlineWalk(c, b)
	}
}

func (r *mdRender) llista(n *html.Node, depth int) {
	ordenada := n.DataAtom == atom.Ol
	num := 1
	if s := attr(n, "start"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			num = v
		}
	}
	r.blockStart()
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Li {
			continue
		}
		marca := "- "
		if ordenada {
			marca = strconv.Itoa(num) + ". "
			num++
		}
		// El text del <li> sense les sub-llistes; les sub-llistes després, indentades.
		sub := &mdRender{base: r.base, nEnllacos: r.nEnllacos}
		var nested []*html.Node
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			if k.Type == html.ElementNode && (k.DataAtom == atom.Ul || k.DataAtom == atom.Ol) {
				nested = append(nested, k)
				continue
			}
			sub.node(k, depth+1)
		}
		r.nEnllacos = sub.nEnllacos
		txt := strings.TrimSpace(collapseLinies(sub.b.String()))
		if txt == "" && len(nested) == 0 {
			continue
		}
		r.b.WriteString(strings.Repeat("  ", depth) + marca + txt + "\n")
		for _, k := range nested {
			r.llista(k, depth+1)
		}
	}
	r.blockEnd()
}

func (r *mdRender) taula(n *html.Node) {
	var files [][]string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					cells = append(cells, strings.ReplaceAll(collapseWS(r.inline(c)), "|", "\\|"))
				}
			}
			if len(cells) > 0 {
				files = append(files, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if len(files) == 0 {
		return
	}
	// Taules de maquetació (una columna, o cel·les enormes): text pla.
	if len(files[0]) == 1 {
		r.blockStart()
		for _, f := range files {
			r.b.WriteString(f[0] + "\n\n")
		}
		r.blockEnd()
		return
	}
	var b strings.Builder
	for i, f := range files {
		b.WriteString("| " + strings.Join(f, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", len(f)) + "\n")
		}
		if i >= 60 {
			b.WriteString("| … |\n")
			break
		}
	}
	r.block("\n\n" + b.String() + "\n")
}

func (r *mdRender) text(s string) {
	if strings.TrimSpace(s) == "" {
		if s != "" {
			r.b.WriteString(" ")
		}
		return
	}
	r.b.WriteString(s)
}

func (r *mdRender) block(s string) { r.b.WriteString(s) }
func (r *mdRender) blockStart()    { r.b.WriteString("\n\n") }
func (r *mdRender) blockEnd()      { r.b.WriteString("\n\n") }

func (r *mdRender) abs(href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if r.base != nil {
		u = r.base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func llenguatgeCodi(pre *html.Node) string {
	cands := []string{attr(pre, "class"), attr(pre, "data-lang")}
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Code {
			cands = append(cands, attr(c, "class"))
		}
	}
	for _, c := range cands {
		for _, cl := range strings.Fields(c) {
			for _, pref := range []string{"language-", "lang-", "highlight-source-", "brush:"} {
				if strings.HasPrefix(cl, pref) {
					return strings.TrimPrefix(cl, pref)
				}
			}
		}
	}
	return ""
}

// netejaMD compacta el markdown: espais repetits, línies buides de més,
// marques buides.
func netejaMD(s string) string {
	var out []string
	dinsCodi := false
	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") {
			dinsCodi = !dinsCodi
			out = append(out, t)
			continue
		}
		if dinsCodi {
			out = append(out, strings.TrimRight(ln, " \t\r"))
			continue
		}
		// Conserva la indentació de les llistes niades, però compacta la resta.
		ind := ""
		if strings.HasPrefix(ln, "  ") && (strings.HasPrefix(t, "- ") || esNumerat(t)) {
			ind = ln[:len(ln)-len(strings.TrimLeft(ln, " "))]
		}
		t = reEspais.ReplaceAllString(t, " ")
		t = strings.ReplaceAll(t, "** **", "")
		t = strings.ReplaceAll(t, "* *", "")
		t = strings.TrimSpace(t)
		out = append(out, ind+t)
	}
	s = strings.Join(out, "\n")
	s = reLinies.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func esNumerat(t string) bool {
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(t) && t[i] == '.' && t[i+1] == ' '
}

func collapseLinies(s string) string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(reEspais.ReplaceAllString(ln, " ")); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// ---- utilitats DOM ----

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func troba(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := troba(c, a); f != nil {
			return f
		}
	}
	return nil
}

func trobaTots(n *html.Node, a atom.Atom) []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == a {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func textDe(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Noscript, atom.Template, atom.Svg:
				return
			case atom.Br, atom.P, atom.Div, atom.Li, atom.Tr:
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func comptaParaules(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) }))
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
