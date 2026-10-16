package tui

import (
	"strings"
	"testing"

	"gregal/internal/brand"
	"gregal/internal/config"
	"gregal/internal/ooxml"
)

func TestBrandingIsAThemeInTheTUI(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	defer func() { ooxml.SetDefaultTheme(""); SetTema("fosc") }()
	p, err := brand.Parse([]byte(`{"name":"Acme","version":"2","colors":{"primary":"#AA0000"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := brand.Install(p); err != nil {
		t.Fatal(err)
	}
	m := &Model{cfg: &config.Config{Theme: "clar"}}
	menu := themeMenu(m)
	found := false
	for _, it := range menu.items {
		if it.val == "brand-acme" && strings.Contains(it.text, "Acme") {
			found = true
		}
	}
	if !found {
		t.Fatalf("theme menu lacks the installed branding: %+v", menu.items)
	}

	if name := m.applyTheme("brand-acme"); name != "Acme" || m.cfg.Brand != "acme" {
		t.Fatalf("applyTheme = %q, cfg.Brand = %q", name, m.cfg.Brand)
	}
	if TemaActiu().Escuma != "#AA0000" || !strings.Contains(Logo(), "ACME") {
		t.Fatalf("active palette %q / logo %q", TemaActiu().Escuma, Logo())
	}
	if m.cfg.Tema() != "brand-acme" || brand.Active() != "acme" {
		t.Fatalf("cfg.Tema() = %q, documents use %q", m.cfg.Tema(), brand.Active())
	}

	// Un tema de casa treu el branding.
	m.applyTheme("gpt")
	if m.cfg.Brand != "" || m.cfg.Theme != "gpt" || brand.Active() != "" || strings.Contains(Logo(), "ACME") {
		t.Fatalf("home theme did not clear the branding: brand=%q theme=%q logo=%q", m.cfg.Brand, m.cfg.Theme, Logo())
	}
}
