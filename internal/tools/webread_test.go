package tools

import (
	"net/url"
	"strings"
	"testing"
)

const paginaArticle = `<!doctype html><html><head>
<title>Com funciona el gregal | El Blog</title>
<meta property="og:title" content="Com funciona el gregal">
<meta name="author" content="Usera">
<meta property="article:published_time" content="2026-09-01T10:00:00Z">
<link rel="canonical" href="https://exemple.cat/gregal">
<style>body{}</style><script>var x=1;</script>
</head><body>
<header><a href="/">Inici</a> <a href="/blog">Blog</a></header>
<nav><ul><li><a href="/a">Secció A</a></li><li><a href="/b">Secció B</a></li></ul></nav>
<div class="cookie-banner">Acceptes les galetes? <button>Sí</button></div>
<main>
<article>
<h1>Com funciona el gregal</h1>
<p>El gregal és un agent de programació que viu en un terminal. Aquesta frase és prou llarga per comptar com a paràgraf, amb comes, punts i tot el que cal.</p>
<p>Té eines de lectura i d'escriptura, i un <a href="/docs/pla">mode pla</a> que explora abans d'actuar. També sap <strong>cercar a la web</strong> i llegir pàgines.</p>
<h2>Instal·lació</h2>
<pre><code class="language-sh">go build .
./gregal init</code></pre>
<ul><li>Primer pas de la llista</li><li>Segon pas amb <code>codi</code></li></ul>
<table><tr><th>Eina</th><th>Permís</th></tr><tr><td>read</td><td>allow</td></tr><tr><td>write</td><td>ask</td></tr></table>
<img src="/foto.png" alt="captura">
<p>Un darrer paràgraf que tanca l'article amb prou paraules perquè el lector el consideri contingut de debò i no soroll.</p>
</article>
</main>
<aside class="sidebar"><h3>Relacionats</h3><a href="/x">Un altre article</a><a href="/y">I un altre</a></aside>
<footer>© 2026 El Blog · <a href="/legal">Avís legal</a></footer>
</body></html>`

func TestReadableArticle(t *testing.T) {
	base, _ := url.Parse("https://exemple.cat/blog/gregal")
	doc := ReadableHTML(paginaArticle, base)
	if doc.Title != "Com funciona el gregal" {
		t.Fatalf("títol: %q", doc.Title)
	}
	if doc.Byline != "Usera" || doc.Published != "2026-09-01" || doc.Canonical != "https://exemple.cat/gregal" {
		t.Fatalf("metadades: %q %q %q", doc.Byline, doc.Published, doc.Canonical)
	}
	md := doc.Markdown
	for _, vol := range []string{"# Com funciona el gregal", "## Instal·lació", "```sh", "./gregal init", "- Primer pas", "`codi`", "| Eina | Permís |", "| read | allow |", "[mode pla](https://exemple.cat/docs/pla)", "**cercar a la web**", "darrer paràgraf"} {
		if !strings.Contains(md, vol) {
			t.Errorf("falta %q a:\n%s", vol, md)
		}
	}
	for _, noVol := range []string{"Secció A", "galetes", "Relacionats", "Avís legal", "var x=1", "foto.png", "captura", "Inici"} {
		if strings.Contains(md, noVol) {
			t.Errorf("soroll %q present a:\n%s", noVol, md)
		}
	}
	if doc.Whole {
		t.Fatal("hauria d'haver trobat el bloc principal")
	}
	if doc.Words < 40 {
		t.Fatalf("poques paraules: %d", doc.Words)
	}
}

func TestReadableSenseArticleAgafaElDivMesDens(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<html><body><div id="menu"><a href="/1">u</a><a href="/2">dos</a><a href="/3">tres</a></div><div class="wrapper"><div class="post-body">`)
	for i := 0; i < 8; i++ {
		b.WriteString("<p>Paràgraf de contingut real, amb prou text i comes, perquè el lector el consideri el cos de la pàgina i no una llista d'enllaços.</p>")
	}
	b.WriteString(`</div></div><div class="comments"><p>Comentari d'un lector, llarg i amb comes, que no hauria de sortir com a contingut principal de la pàgina.</p></div></body></html>`)
	doc := ReadableHTML(b.String(), nil)
	if strings.Contains(doc.Markdown, "Comentari d'un lector") {
		t.Fatalf("els comentaris no són contingut:\n%s", doc.Markdown)
	}
	if strings.Count(doc.Markdown, "Paràgraf de contingut real") != 8 {
		t.Fatalf("paràgrafs perduts:\n%s", doc.Markdown)
	}
}

func TestReadablePaginaCurtaAgafaTot(t *testing.T) {
	doc := ReadableHTML(`<html><head><title>Hola</title></head><body><h1>Títol</h1><p>Text curt amb <b>negreta</b>.</p></body></html>`, nil)
	if !strings.Contains(doc.Markdown, "Títol") || !strings.Contains(doc.Markdown, "negreta") {
		t.Fatalf("text perdut: %q", doc.Markdown)
	}
}

func TestRenderRetallINota(t *testing.T) {
	p := &pagina{URL: "https://x.cat/p", Title: "T", Words: 3, Body: strings.Repeat("paraula ", 2000)}
	out := p.render(WebFetchOpts{MaxChars: 500})
	if len(out) > 700 || !strings.Contains(out, "retallat") || !strings.Contains(out, "find:") {
		t.Fatalf("retall inesperat (%d): %.120s", len(out), out)
	}
}

func TestCercaParagrafs(t *testing.T) {
	body := "Primer paràgraf sobre gats.\n\nSegon paràgraf sobre gossos i més gossos.\n\nTercer sobre peixos."
	out := cercaParagrafs(body, "gossos")
	if !strings.Contains(out, "[¶2]") || strings.Contains(out, "gats") {
		t.Fatalf("cerca: %q", out)
	}
	// Frase sencera absent → paraules soltes.
	out = cercaParagrafs(body, "peixos grossos")
	if !strings.Contains(out, "[¶3]") {
		t.Fatalf("cerca per paraules: %q", out)
	}
	if out = cercaParagrafs(body, "zzz"); !strings.Contains(out, "cap paràgraf") {
		t.Fatalf("sense coincidència: %q", out)
	}
}

func TestParseDuckDuckGo(t *testing.T) {
	src := `<html><body><div class="results">
<div class="result"><h2 class="result__title"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fcharmbracelet%2Fbubbletea&amp;rut=abc">Bubble Tea</a></h2>
<a class="result__snippet" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fcharmbracelet%2Fbubbletea">A powerful little TUI framework</a></div>
<div class="result"><a class="result__a" href="https://example.com/x">Exemple</a><div class="result__snippet">Resum dos</div></div>
</div></body></html>`
	hits, err := parseDuckDuckGo(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].URL != "https://github.com/charmbracelet/bubbletea" || hits[0].Title != "Bubble Tea" ||
		hits[0].Snippet != "A powerful little TUI framework" || hits[1].URL != "https://example.com/x" || hits[1].Snippet != "Resum dos" {
		t.Fatalf("hits: %+v", hits)
	}
}

func TestParseBingRSS(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8" ?><rss version="2.0"><channel><title>Bing: q</title>
<item><title>T&amp;1 &lt;b&gt;bold&lt;/b&gt;</title><link>https://a.cat/1</link><description>D1</description></item>
<item><title>T2</title><link>https://b.cat/2</link><description>D2 &amp; més</description></item>
</channel></rss>`
	hits, err := parseBingRSS([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Title != "T&1 bold" || hits[1].Snippet != "D2 & més" || hits[1].URL != "https://b.cat/2" {
		t.Fatalf("hits: %+v", hits)
	}
}

func TestFusionaDedupIRang(t *testing.T) {
	a := []SearchHit{{Title: "A", URL: "https://www.x.cat/a/"}, {Title: "B", URL: "https://x.cat/b"}}
	b := []SearchHit{{Title: "B llarg", URL: "https://x.cat/b?utm_source=z"}, {Title: "C", URL: "https://x.cat/c"}}
	out := fusiona([][]SearchHit{a, b}, 10)
	if len(out) != 3 {
		t.Fatalf("dedup: %+v", out)
	}
	// B surt a les dues llistes (1/2 + 1/1) i passa davant d'A (1/1).
	if out[0].Title != "B llarg" || out[1].Title != "A" || out[2].Title != "C" {
		t.Fatalf("ordre: %+v", out)
	}
	if got := fusiona([][]SearchHit{a, b}, 2); len(got) != 2 {
		t.Fatalf("count: %+v", got)
	}
}

func TestWebSearchFusionaMotors(t *testing.T) {
	// Dos motors falsos: un SearXNG i el fallback pel DDG (via base
	// redirigida). El Bing apunta a un port tancat: ha de fallar ràpid
	// sense tombar la cerca.
	WebCacheClear()
	t.Cleanup(WebCacheClear)
	srv := newFakeSearch(t)
	defer srv.Close()
	t.Setenv("SEARXNG_URL", srv.URL+"/searx")
	oldD, oldB := duckduckgoBase, bingBase
	duckduckgoBase, bingBase = srv.URL+"/ddg", "http://127.0.0.1:1/bing"
	defer func() { duckduckgoBase, bingBase = oldD, oldB }()
	out, err := WebSearch("gregal", 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "via searxng+duckduckgo") || !strings.Contains(out, "https://x.cat/comu") || !strings.Contains(out, "https://x.cat/nomes-ddg") {
		t.Fatalf("fusió: %s", out)
	}
	// El resultat comú (als dos motors) ha de sortir primer.
	if i, j := strings.Index(out, "x.cat/comu"), strings.Index(out, "x.cat/nomes-searx"); i > j {
		t.Fatalf("ordre: %s", out)
	}
}
