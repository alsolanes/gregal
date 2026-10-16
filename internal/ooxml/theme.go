package ooxml

import (
	"sort"
	"strings"
)

// Theme és una paleta i una parella de tipus de lletra coherents per a
// docx i pptx. Colors en hex sense «#». Els tipus de lletra són de la
// llista segura (es renderitzen igual a Office i a LibreOffice).
type Theme struct {
	Name        string
	HeadingFont string
	BodyFont    string
	Dark        string // fons fosc / text principal dels títols
	Primary     string // color dominant
	Accent      string // color d'accent (xifres, destacats)
	Accent2     string
	Light       string // fons clar suau (capçaleres de taula, caixes)
	Text        string // text de cos
	Muted       string // text secundari
}

var themes = map[string]Theme{
	"default": {Name: "default", HeadingFont: "Calibri", BodyFont: "Calibri",
		Dark: "1F2A44", Primary: "1F4E79", Accent: "2E86AB", Accent2: "F18F01", Light: "EAF1F8", Text: "222222", Muted: "6B7280"},
	"modern": {Name: "modern", HeadingFont: "Arial", BodyFont: "Arial",
		Dark: "1E1E24", Primary: "2B2D42", Accent: "EF476F", Accent2: "06D6A0", Light: "F1F1F4", Text: "202124", Muted: "6E6E7A"},
	"nature": {Name: "nature", HeadingFont: "Calibri", BodyFont: "Calibri",
		Dark: "1B3A2F", Primary: "2D6A4F", Accent: "52B788", Accent2: "D4A373", Light: "EEF6F0", Text: "1F2D27", Muted: "667A70"},
	"elegant": {Name: "elegant", HeadingFont: "Cambria", BodyFont: "Calibri",
		Dark: "2B1B24", Primary: "6D213C", Accent: "B08D57", Accent2: "3E5C76", Light: "F6F0EE", Text: "2A2A2A", Muted: "7A6E70"},
	"mono": {Name: "mono", HeadingFont: "Arial", BodyFont: "Arial",
		Dark: "111111", Primary: "333333", Accent: "0070C0", Accent2: "7F7F7F", Light: "F2F2F2", Text: "1A1A1A", Muted: "6F6F6F"},
}

// ThemeNames torna els noms disponibles.
func ThemeNames() []string {
	var out []string
	for k := range themes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// GetTheme torna el tema pel nom (o el per defecte). Admet sobreescriure
// colors amb «primary=…,accent=…» (hex) i tipus de lletra amb
// «font=Arial» / «heading_font=Cambria».
func GetTheme(spec string) Theme {
	name, rest, _ := strings.Cut(strings.TrimSpace(spec), ":")
	t, ok := themes[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		t = themes["default"]
		if strings.Contains(name, "=") {
			rest = name
		}
	}
	for _, kv := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			continue
		}
		v = strings.TrimPrefix(strings.TrimSpace(v), "#")
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "primary":
			t.Primary = hexOr(v, t.Primary)
		case "accent":
			t.Accent = hexOr(v, t.Accent)
		case "accent2":
			t.Accent2 = hexOr(v, t.Accent2)
		case "dark":
			t.Dark = hexOr(v, t.Dark)
		case "light":
			t.Light = hexOr(v, t.Light)
		case "text":
			t.Text = hexOr(v, t.Text)
		case "font", "body_font":
			if v != "" {
				t.BodyFont = v
				if strings.ToLower(k) == "font" {
					t.HeadingFont = v
				}
			}
		case "heading_font":
			if v != "" {
				t.HeadingFont = v
			}
		}
	}
	return t
}

func hexOr(v, def string) string {
	if len(v) != 6 {
		return def
	}
	for _, r := range v {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return def
		}
	}
	return strings.ToUpper(v)
}

// SafeFonts són els tipus de lletra que Office i LibreOffice
// renderitzen igual (la QA visual no enganya).
var SafeFonts = []string{"Arial", "Calibri", "Cambria", "Times New Roman", "Courier New", "Georgia", "Verdana"}
