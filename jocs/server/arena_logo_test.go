package server

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestArenaLogoValidationAndHash(t *testing.T) {
	s := cornerTestServer(t)
	if s.arenaEvent().LogoHash != "" {
		t.Fatal("no logo yet")
	}
	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG fake"))
	if err := s.saveArenaLogo(png); err != nil {
		t.Fatal(err)
	}
	if s.arenaLogo() != png || len(s.arenaEvent().LogoHash) != 12 {
		t.Fatalf("logo %q hash %q", s.arenaLogo(), s.arenaEvent().LogoHash)
	}
	for _, bad := range []string{
		"https://example.com/logo.png",
		"data:text/html;base64,PGgxPg==",
		"data:image/png;base64,***",
		"data:image/png;base64," + strings.Repeat("A", arenaLogoMax),
	} {
		if err := s.saveArenaLogo(bad); err == nil {
			t.Errorf("accepted %q", bad[:min(len(bad), 40)])
		}
	}
	if s.arenaLogo() != png {
		t.Fatal("a rejected logo replaced the good one")
	}
	if err := s.saveArenaLogo(""); err != nil || s.arenaLogo() != "" || s.arenaEvent().LogoHash != "" {
		t.Fatalf("remove: %v %q", err, s.arenaLogo())
	}
}
