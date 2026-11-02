package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un fitxer enviat al xat ha de poder-se baixar des del mòbil sense capçaleres
// de sessió: la clau del camí ja és el permís.
func TestFitxerEnviatEsBaixaAmbElSeuEnllac(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := goalTestServer(t)

	origen := filepath.Join(t.TempDir(), "Gregal-Vents-1.55.0-arm64-v8a.apk")
	if err := os.WriteFile(origen, []byte("APK de prova"), 0o644); err != nil {
		t.Fatal(err)
	}
	enllac, nom, mida, err := s.publicaFitxer(context.Background(), origen)
	if err != nil {
		t.Fatalf("publicaFitxer: %v", err)
	}
	if nom != "Gregal-Vents-1.55.0-arm64-v8a.apk" || mida != int64(len("APK de prova")) {
		t.Fatalf("nom=%q mida=%d", nom, mida)
	}
	if !strings.Contains(enllac, "/f/") || !strings.HasSuffix(enllac, "/"+nom) {
		t.Fatalf("enllaç estrany: %q", enllac)
	}

	cami := enllac[strings.Index(enllac, "/f/"):]
	rec := httptest.NewRecorder()
	s.handleFitxer(rec, httptest.NewRequest(http.MethodGet, cami, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("codi %d", rec.Code)
	}
	if rec.Body.String() != "APK de prova" {
		t.Fatalf("cos %q", rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, nom) {
		t.Fatalf("Content-Disposition %q", cd)
	}
}

func TestFitxerAmbClauInventadaNoSurt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := goalTestServer(t)
	for _, cami := range []string{
		"/f/000000000000000000000000/inexistent.apk",
		"/f/curt/x.apk",
		"/f/000000000000000000000000/../secret.txt",
	} {
		rec := httptest.NewRecorder()
		s.handleFitxer(rec, httptest.NewRequest(http.MethodGet, cami, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s -> codi %d", cami, rec.Code)
		}
	}
}
