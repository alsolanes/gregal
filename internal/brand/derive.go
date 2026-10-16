package brand

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gregal/internal/ooxml"
	"gregal/internal/tema"
)

// ThemeName és el nom de tema (i el data-theme del web) d'un paquet.
func ThemeName(id string) string { return tema.PrefixBrand + id }

// Tema és la paleta de l'app per al paquet: el tema de base (fosc o clar)
// amb els colors del paquet a sobre, i els tons que no porta derivats
// d'aquests perquè tot lligui.
func (p *Pack) Tema() tema.Tema {
	light := p.Mode == "light"
	t := tema.Fosc()
	tint, shade := "#FFFFFF", "#000000"
	if light {
		t = tema.Clar()
	}
	t.Nom = ThemeName(p.ID)
	t.Clar = light
	c := p.Colors

	if c.Background != "" {
		t.Fons = c.Background
		if light {
			t.Abisme = mix(c.Background, shade, .035)
		} else {
			t.Abisme = mix(c.Background, shade, .22)
		}
		t.Night = mix(c.Background, pick(light, shade, tint), .05)
	}
	if c.Surface != "" {
		t.Night = c.Surface
	}
	if c.Background != "" || c.Surface != "" {
		t.SuperficieAlta = mix(t.Night, pick(light, shade, tint), .06)
	}
	if c.Text != "" {
		t.Ink = c.Text
		t.Text = mix(c.Text, t.Fons, .1)
	}
	if c.Muted != "" {
		t.Slate = c.Muted
		t.Faint = mix(c.Muted, t.Fons, .35)
	}
	t.Escuma = c.Primary
	t.EscumaClara = mix(c.Primary, pick(light, shade, tint), .18)
	t.Algua = mix(c.Primary, t.Fons, .6)
	t.BoxIdle = mix(c.Primary, t.Fons, .45)
	t.Vora = t.Slate
	if c.Accent != "" {
		t.Arena = c.Accent
	}
	if c.Secondary != "" {
		t.Onada = c.Secondary
	}
	if c.Success != "" {
		t.Green = c.Success
	}
	if c.Danger != "" {
		t.Red = c.Danger
	}
	t.Sintaxi.Funcio = c.Primary
	return t
}

func pick(light bool, a, b string) string {
	if light {
		return a
	}
	return b
}

// Office és el tema dels documents (docx, pptx, xlsx). Els documents
// van sobre blanc, sigui quin sigui el mode de l'app: del paquet en surten
// els colors de marca i els tipus de lletra, i el text és fosc.
func (p *Pack) Office() ooxml.Theme {
	c := p.Colors
	accent := first(c.Accent, c.Secondary, mix(c.Primary, "#FFFFFF", .35))
	accent2 := first(c.Secondary, c.Accent, mix(c.Primary, "#000000", .3))
	dark := mix(c.Primary, "#000000", .55)
	text := "#222222"
	muted := "#6B7280"
	if p.Mode == "light" {
		if c.Text != "" {
			text = c.Text
			dark = c.Text
		}
		if c.Muted != "" {
			muted = c.Muted
		}
	}
	body := firstFamily(p.Fonts.Body)
	if body == "" {
		body = "Calibri"
	}
	heading := firstFamily(p.Fonts.Heading)
	if heading == "" {
		heading = body
	}
	t := ooxml.Theme{
		Name: ThemeName(p.ID), HeadingFont: heading, BodyFont: body,
		Dark: hex6(dark), Primary: hex6(c.Primary), Accent: hex6(accent), Accent2: hex6(accent2),
		Light: hex6(mix(c.Primary, "#FFFFFF", .9)), Text: hex6(text), Muted: hex6(muted),
	}
	if mime, raw, err := p.LogoData(); err == nil && len(raw) > 0 {
		switch mime {
		case "image/png":
			t.Logo, t.LogoExt = raw, "png"
		case "image/jpeg":
			t.Logo, t.LogoExt = raw, "jpeg"
		}
	}
	return t
}

// ChartColors són els colors de les sèries dels gràfics: els del paquet o,
// si no en porta, els de marca.
func (p *Pack) ChartColors() []string {
	if len(p.Colors.Chart) > 0 {
		return append([]string(nil), p.Colors.Chart...)
	}
	c := p.Colors
	out := []string{c.Primary}
	for _, v := range []string{c.Accent, c.Secondary, c.Success, c.Danger} {
		if v != "" {
			out = append(out, v)
		}
	}
	for len(out) < 4 {
		out = append(out, mix(c.Primary, pick(p.Mode == "light", "#000000", "#FFFFFF"), .25*float64(len(out))))
	}
	return out
}

// CSS és el bloc :root[data-theme="brand-<id>"] del paquet per al web i
// l'escriptori: els mateixos tokens que els temes de casa més els tipus de
// lletra i els colors dels gràfics.
func (p *Pack) CSS() string {
	t := p.Tema()
	toks := tema.CSSColors(t)
	if f := fontStack(p.Fonts.Body, `Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif`); f != "" {
		toks = append(toks, "--font:"+f)
	}
	if f := fontStack(p.Fonts.Heading, "var(--font)"); f != "" {
		toks = append(toks, "--font-heading:"+f)
	}
	if f := fontStack(p.Fonts.Mono, `ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace`); f != "" {
		toks = append(toks, "--mono:"+f)
	}
	for i, c := range p.ChartColors() {
		toks = append(toks, fmt.Sprintf("--chart-%d:%s", i+1, c))
	}
	var b strings.Builder
	b.WriteString(`:root[data-theme="` + t.Nom + `"] {` + "\n")
	for _, tk := range toks {
		b.WriteString("  " + tk + ";\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// Swatch és un color amb nom, per a la previsualització.
type Swatch struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

// Swatches són els colors que porta el paquet (els que hi ha escrits, no
// els derivats), en l'ordre en què es mostren.
func (p *Pack) Swatches() []Swatch {
	c := p.Colors
	var out []Swatch
	for _, s := range []Swatch{
		{"primary", c.Primary}, {"accent", c.Accent}, {"secondary", c.Secondary},
		{"background", c.Background}, {"surface", c.Surface}, {"text", c.Text},
		{"muted", c.Muted}, {"success", c.Success}, {"danger", c.Danger},
	} {
		if s.Hex != "" {
			out = append(out, s)
		}
	}
	for i, h := range c.Chart {
		out = append(out, Swatch{fmt.Sprintf("chart %d", i+1), h})
	}
	return out
}

// fontStack fa una llista CSS amb els noms del paquet entre cometes (si
// cal) i el fallback al final. Buit = "" (no se sobreescriu res).
func fontStack(spec, fallback string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	var parts []string
	for _, f := range strings.Split(spec, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.HasPrefix(f, `"`) && !strings.HasPrefix(f, "'") && strings.ContainsAny(f, " ") {
			f = `"` + strings.ReplaceAll(f, `"`, "") + `"`
		}
		parts = append(parts, f)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",") + "," + fallback
}

// firstFamily és el primer nom d'una llista de tipus de lletra, sense
// cometes (els documents en volen un de sol).
func firstFamily(spec string) string {
	f, _, _ := strings.Cut(spec, ",")
	return strings.Trim(strings.TrimSpace(f), `"'`)
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func hex6(h string) string { return strings.ToUpper(strings.TrimPrefix(h, "#")) }

func rgb(h string) (float64, float64, float64) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 0, 0, 0
	}
	v := func(s string) float64 {
		n, _ := strconv.ParseUint(s, 16, 8)
		return float64(n)
	}
	return v(h[0:2]), v(h[2:4]), v(h[4:6])
}

// mix barreja a amb b: t=0 és a, t=1 és b.
func mix(a, b string, t float64) string {
	ar, ag, ab := rgb(a)
	br, bg, bb := rgb(b)
	c := func(x, y float64) int { return int(math.Round(x + (y-x)*t)) }
	return fmt.Sprintf("#%02X%02X%02X", c(ar, br), c(ag, bg), c(ab, bb))
}
