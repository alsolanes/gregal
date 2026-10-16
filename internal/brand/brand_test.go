package brand

import (
	"encoding/base64"
	"strings"
	"testing"

	"gregal/internal/ooxml"
	"gregal/internal/tema"
)

// png1x1 és un PNG vàlid d'un píxel.
var png1x1 = "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1F, 0x15, 0xC4, 0x89,
})

func TestParseNormalizes(t *testing.T) {
	p, err := Parse([]byte(`{"name":"Clear Peaks","mode":"light","colors":{"primary":"#0a6","accent":"ff6b5b","chart":["#111","#222222"]},"fonts":{"body":"The Future, Arial"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "clear-peaks" || p.Format != Format || p.Version != "1" || p.Mode != "light" {
		t.Fatalf("normalized pack = %+v", p)
	}
	if p.Colors.Primary != "#00AA66" || p.Colors.Accent != "#FF6B5B" || p.Colors.Chart[0] != "#111111" {
		t.Fatalf("colours = %+v", p.Colors)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"no primary":    `{"name":"x","colors":{}}`,
		"no name":       `{"colors":{"primary":"#000"}}`,
		"bad colour":    `{"name":"x","colors":{"primary":"red"}}`,
		"css injection": `{"name":"x","colors":{"primary":"#000"},"fonts":{"body":"Arial;}body{display:none"}}`,
		"bad mode":      `{"name":"x","mode":"neon","colors":{"primary":"#000"}}`,
		"html in name":  `{"name":"<b>x</b>","colors":{"primary":"#000"}}`,
		"bad format":    `{"format":"other/9","name":"x","colors":{"primary":"#000"}}`,
		"svg script": `{"name":"x","colors":{"primary":"#000"},"logo":"data:image/svg+xml;base64,` +
			base64.StdEncoding.EncodeToString([]byte(`<svg><script>alert(1)</script></svg>`)) + `"}`,
		"svg onload": `{"name":"x","colors":{"primary":"#000"},"logo":"data:image/svg+xml;base64,` +
			base64.StdEncoding.EncodeToString([]byte(`<svg onload="x()"></svg>`)) + `"}`,
		"logo not data url": `{"name":"x","colors":{"primary":"#000"},"logo":"https://example.com/a.png"}`,
		"logo gif":          `{"name":"x","colors":{"primary":"#000"},"logo":"data:image/gif;base64,R0lGOA=="}`,
		"not json":          `hello`,
	}
	for name, in := range cases {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s: accepted %s", name, in)
		}
	}
}

func TestTemaAndOffice(t *testing.T) {
	p, err := Parse([]byte(`{"name":"Acme","mode":"dark","colors":{"primary":"#FF0000","background":"#101010","accent":"#00FF00"},"fonts":{"body":"Georgia","heading":"\"The Future\""},"logo":"` + png1x1 + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	tm := p.Tema()
	if tm.Nom != "brand-acme" || tm.Clar || tm.Escuma != "#FF0000" || tm.Fons != "#101010" || tm.Arena != "#00FF00" {
		t.Fatalf("tema = %+v", tm)
	}
	of := p.Office()
	if of.Primary != "FF0000" || of.BodyFont != "Georgia" || of.HeadingFont != "The Future" || of.LogoExt != "png" || len(of.Logo) == 0 {
		t.Fatalf("office = %+v", of)
	}
	css := p.CSS()
	for _, want := range []string{`:root[data-theme="brand-acme"]`, "--accent:#FF0000", `--font:Georgia,`, `--font-heading:"The Future",`, "--chart-1:#FF0000"} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS lacks %q:\n%s", want, css)
		}
	}
}

func TestInstallSetupActivateRemove(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	defer ooxml.SetDefaultTheme("")
	p, err := Parse([]byte(`{"name":"Acme","colors":{"primary":"#123456"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(p); err != nil {
		t.Fatal(err)
	}
	if !tema.Existeix("brand-acme") || tema.Per("brand-acme").Escuma != "#123456" {
		t.Fatal("installed pack is not a registered theme")
	}
	if got := List(); len(got) != 1 || got[0].ID != "acme" {
		t.Fatalf("List = %+v", got)
	}
	Activate("acme")
	if Active() != "acme" || ooxml.GetTheme("").Primary != "123456" {
		t.Fatalf("active = %q, empty-spec theme = %+v", Active(), ooxml.GetTheme(""))
	}
	if ooxml.GetTheme("default").Primary == "123456" {
		t.Fatal("an explicit default theme must not follow the branding")
	}
	if err := Remove("acme"); err != nil {
		t.Fatal(err)
	}
	if tema.Existeix("brand-acme") || Active() != "" || ooxml.GetTheme("").Name != "default" {
		t.Fatal("removed pack is still registered or active")
	}
	if err := Remove("acme"); err != ErrNotFound {
		t.Fatalf("second remove = %v", err)
	}
}
