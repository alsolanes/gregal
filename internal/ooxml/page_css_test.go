package ooxml

import (
	"strconv"
	"strings"
	"testing"
)

// La pàgina del web (PageCSS) i el docx que exporta un document (NewDocx
// sense tema: el branding actiu) han de dir el mateix: tipus de lletra,
// mides, colors i pàgina.
func TestPageCSSReadsTheDocxExportStyles(t *testing.T) {
	RegisterTheme(Theme{Name: "pagina-test", HeadingFont: "Georgia", BodyFont: "Verdana",
		Dark: "101010", Primary: "123456", Accent: "ABCDEF", Accent2: "FEDCBA", Light: "F0F0F0", Text: "202020", Muted: "707070"})
	SetDefaultTheme("pagina-test")
	defer UnregisterTheme("pagina-test")

	css := PageCSS(GetTheme(""))
	pk := NewDocx(DocxOptions{Title: "x"}).Pkg
	styles, err := pk.XML("word/styles.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := pk.XML("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	s, d := styles.String(), doc.String()
	style := func(id string) string {
		_, rest, ok := strings.Cut(s, `w:styleId="`+id+`"`)
		if !ok {
			t.Fatalf("no style %s", id)
		}
		body, _, _ := strings.Cut(rest, "</w:style>")
		return body
	}
	twentieths := func(v string, k float64) string {
		return strconv.Itoa(int(mustFloat(t, css[v])*k + 0.5))
	}
	color := func(v string) string { return strings.TrimPrefix(css[v], "#") }
	font := func(v string) string { return `w:ascii="` + strings.Trim(css[v], `"`) + `"` }

	for id, vars := range map[string][2]string{
		"Heading1": {"--doc-h1-size", "--doc-h1-color"},
		"Heading2": {"--doc-h2-size", "--doc-h2-color"},
		"Heading3": {"--doc-h3-size", "--doc-h3-color"},
	} {
		h := style(id)
		for _, want := range []string{`w:sz w:val="` + twentieths(vars[0], 2) + `"`, `w:color w:val="` + color(vars[1]) + `"`, font("--doc-heading-font")} {
			if !strings.Contains(h, want) {
				t.Errorf("%s: want %s in %s", id, want, h)
			}
		}
	}
	for _, want := range []string{
		font("--doc-body-font"),
		`w:color w:val="` + color("--doc-text") + `"/><w:sz w:val="` + twentieths("--doc-body-size", 2) + `"`,
		`<w:spacing w:after="` + twentieths("--doc-space-after", 20) + `"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("docDefaults: want %s", want)
		}
	}
	q := style("Quote")
	if !strings.Contains(q, `w:color="`+color("--doc-accent")+`"`) || !strings.Contains(q, `w:color w:val="`+color("--doc-muted")+`"`) {
		t.Errorf("Quote: %s", q)
	}
	c := style("Code")
	if !strings.Contains(c, `w:fill="`+color("--doc-light")+`"`) || !strings.Contains(c, font("--doc-code-font")) || !strings.Contains(c, `w:sz w:val="`+twentieths("--doc-code-size", 2)+`"`) {
		t.Errorf("Code: %s", c)
	}
	if !strings.Contains(style("TableGrid"), `w:color="`+color("--doc-table-border")+`"`) {
		t.Error("TableGrid border")
	}
	page := `w:pgSz w:w="` + twentieths("--doc-page-w", 20) + `"`
	margins := `w:top="` + twentieths("--doc-margin-top", 20) + `" w:right="` + twentieths("--doc-margin-side", 20) + `"`
	if !strings.Contains(d, page) || !strings.Contains(d, margins) {
		t.Errorf("page: want %s and %s", page, margins)
	}
	if css["--doc-primary"] != "#123456" || css["--doc-body-font"] != `"Verdana"` {
		t.Errorf("not the active theme: %v", css)
	}
}

func mustFloat(t *testing.T, v string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		t.Fatalf("%q: %v", v, err)
	}
	return f
}
