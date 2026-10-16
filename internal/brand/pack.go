// Package brand són els paquets de branding: un fitxer JSON amb colors,
// tipus de lletra i logo que es fa servir a tot arreu (web, escriptori,
// TUI) i als documents que genera Gregal (docx, pptx, xlsx, exportacions).
//
// Un paquet és un sol fitxer (.gregal-brand.json) perquè es pugui passar
// de mà en mà i arrossegar a la pantalla de Brandings: el logo hi va dins
// com a data URL. Només el color primari és obligatori; la resta surt del
// tema de base (fosc o clar) i es pot anar afinant.
package brand

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Format és l'identificador del format dels fitxers.
const Format = "gregal-brand/1"

// MaxFile és la mida màxima d'un fitxer de paquet (logo inclòs).
const MaxFile = 1 << 20

// MaxLogo és la mida màxima del logo un cop descodificat.
const MaxLogo = 512 << 10

// Colors del paquet, en hex "#RRGGBB". Només Primary és obligatori.
type Colors struct {
	Primary    string   `json:"primary"`
	Accent     string   `json:"accent,omitempty"`
	Secondary  string   `json:"secondary,omitempty"`
	Background string   `json:"background,omitempty"`
	Surface    string   `json:"surface,omitempty"`
	Text       string   `json:"text,omitempty"`
	Muted      string   `json:"muted,omitempty"`
	Success    string   `json:"success,omitempty"`
	Danger     string   `json:"danger,omitempty"`
	Chart      []string `json:"chart,omitempty"`
}

// Fonts són noms de família (o llistes CSS curtes). Buit = el de sempre.
type Fonts struct {
	Body    string `json:"body,omitempty"`
	Heading string `json:"heading,omitempty"`
	Mono    string `json:"mono,omitempty"`
}

// Pack és un paquet de branding.
type Pack struct {
	Format  string `json:"format"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Author  string `json:"author,omitempty"`
	// Mode és el tema de base: "dark" o "light".
	Mode   string `json:"mode"`
	Colors Colors `json:"colors"`
	Fonts  Fonts  `json:"fonts,omitempty"`
	// Logo és una data URL (PNG, JPEG, WebP o SVG).
	Logo string `json:"logo,omitempty"`
}

var (
	reHex  = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	reID   = regexp.MustCompile(`[^a-z0-9]+`)
	reFont = regexp.MustCompile(`^[\p{L}\p{N} _\-.,'"]{1,120}$`)
	reVer  = regexp.MustCompile(`^[0-9A-Za-z.\-+]{1,32}$`)
)

var logoMimes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/svg+xml": true}

// Parse llegeix i valida un fitxer de paquet. El paquet que torna ja està
// normalitzat (colors en majúscula amb #, id en slug).
func Parse(data []byte) (*Pack, error) {
	if len(data) > MaxFile {
		return nil, fmt.Errorf("the file is larger than %d KB", MaxFile>>10)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var p Pack
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("not a branding file: %v", err)
	}
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Normalize valida el paquet i el deixa en forma canònica.
func (p *Pack) Normalize() error {
	if f := strings.TrimSpace(p.Format); f != "" && f != Format {
		return fmt.Errorf("unsupported format %q (expected %q)", f, Format)
	}
	p.Format = Format
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len([]rune(p.Name)) > 60 {
		return fmt.Errorf("name is required (up to 60 characters)")
	}
	if strings.ContainsAny(p.Name, "<>") {
		return fmt.Errorf("name cannot contain < or >")
	}
	id := strings.TrimSpace(p.ID)
	if id == "" {
		id = p.Name
	}
	p.ID = Slug(id)
	if p.ID == "" {
		return fmt.Errorf("id must contain letters or digits")
	}
	p.Version = strings.TrimSpace(p.Version)
	if p.Version == "" {
		p.Version = "1"
	}
	if !reVer.MatchString(p.Version) {
		return fmt.Errorf("invalid version %q", p.Version)
	}
	p.Author = strings.TrimSpace(p.Author)
	if len([]rune(p.Author)) > 80 || strings.ContainsAny(p.Author, "<>") {
		return fmt.Errorf("invalid author")
	}
	switch strings.ToLower(strings.TrimSpace(p.Mode)) {
	case "", "dark", "fosc":
		p.Mode = "dark"
	case "light", "clar":
		p.Mode = "light"
	default:
		return fmt.Errorf("mode must be dark or light")
	}
	c := &p.Colors
	if strings.TrimSpace(c.Primary) == "" {
		return fmt.Errorf("colors.primary is required")
	}
	for _, f := range []struct {
		name string
		v    *string
	}{
		{"primary", &c.Primary}, {"accent", &c.Accent}, {"secondary", &c.Secondary},
		{"background", &c.Background}, {"surface", &c.Surface}, {"text", &c.Text},
		{"muted", &c.Muted}, {"success", &c.Success}, {"danger", &c.Danger},
	} {
		h, err := normHex(*f.v)
		if err != nil {
			return fmt.Errorf("colors.%s: %v", f.name, err)
		}
		*f.v = h
	}
	if len(c.Chart) > 8 {
		return fmt.Errorf("colors.chart: up to 8 colours")
	}
	for i, v := range c.Chart {
		h, err := normHex(v)
		if err != nil || h == "" {
			return fmt.Errorf("colors.chart[%d]: invalid colour %q", i, v)
		}
		c.Chart[i] = h
	}
	for _, f := range []struct {
		name string
		v    *string
	}{{"body", &p.Fonts.Body}, {"heading", &p.Fonts.Heading}, {"mono", &p.Fonts.Mono}} {
		*f.v = strings.TrimSpace(*f.v)
		if *f.v != "" && !reFont.MatchString(*f.v) {
			return fmt.Errorf("fonts.%s: only letters, digits, spaces, commas and quotes", f.name)
		}
	}
	p.Logo = strings.TrimSpace(p.Logo)
	if p.Logo != "" {
		if _, _, err := p.LogoData(); err != nil {
			return err
		}
	}
	return nil
}

// LogoData descodifica el logo: tipus MIME i bytes.
func (p *Pack) LogoData() (string, []byte, error) {
	if p.Logo == "" {
		return "", nil, nil
	}
	head, b64, ok := strings.Cut(p.Logo, ",")
	if !ok || !strings.HasPrefix(head, "data:") || !strings.HasSuffix(head, ";base64") {
		return "", nil, fmt.Errorf("logo must be a base64 data URL")
	}
	mime := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(head, "data:"), ";base64"))
	if !logoMimes[mime] {
		return "", nil, fmt.Errorf("logo must be PNG, JPEG, WebP or SVG")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", nil, fmt.Errorf("logo: invalid base64")
	}
	if len(raw) > MaxLogo {
		return "", nil, fmt.Errorf("logo is larger than %d KB", MaxLogo>>10)
	}
	if mime == "image/svg+xml" && unsafeSVG(raw) {
		return "", nil, fmt.Errorf("logo: SVG with scripts or external references is not allowed")
	}
	return mime, raw, nil
}

// unsafeSVG rebutja els SVG amb scripts, gestors d'esdeveniments o
// referències de fora. El logo es mostra com a <img> (on no s'executa
// res), però el fitxer també es pot obrir sol.
func unsafeSVG(b []byte) bool {
	s := strings.ToLower(string(b))
	for _, bad := range []string{"<script", "javascript:", "<foreignobject", "<iframe", "<embed", "<object"} {
		if strings.Contains(s, bad) {
			return true
		}
	}
	return reSVGOn.MatchString(s) || reSVGHref.MatchString(s)
}

var (
	reSVGOn   = regexp.MustCompile(`\son[a-z]+\s*=`)
	reSVGHref = regexp.MustCompile(`(?:xlink:)?href\s*=\s*["']\s*(?:https?:|//)`)
)

// Slug passa un text a id: minúscules, xifres i guions.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("à", "a", "á", "a", "è", "e", "é", "e", "í", "i", "ï", "i",
		"ò", "o", "ó", "o", "ú", "u", "ü", "u", "ç", "c", "ñ", "n", "·", "").Replace(s)
	s = strings.Trim(reID.ReplaceAllString(s, "-"), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

func normHex(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	m := reHex.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("invalid colour %q (use #RRGGBB)", v)
	}
	h := strings.ToUpper(m[1])
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	return "#" + h, nil
}

// CompareVersions compara dues versions per trossos ("1.10" > "1.9"):
// -1, 0 o 1. Els trossos numèrics es comparen com a nombres; la resta, com
// a text.
func CompareVersions(a, b string) int {
	split := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	}
	pa, pb := split(a), split(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y string
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		if x == "" {
			nx, ex = 0, nil
		}
		if y == "" {
			ny, ey = 0, nil
		}
		switch {
		case ex == nil && ey == nil:
			if nx != ny {
				if nx < ny {
					return -1
				}
				return 1
			}
		case x != y:
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Marshal escriu el paquet com a fitxer (indentat, per llegir-lo i fer-ne
// diffs).
func (p *Pack) Marshal() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
