package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClientIPNoEsCreuLaCapcaleraDeQualsevol: X-Forwarded-For només compta
// darrere d'un proxy local, i aleshores val l'última adreça (la que hi posa
// el proxy), no la primera, que l'escriu el client.
func TestClientIPNoEsCreuLaCapcaleraDeQualsevol(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("des de fora, la capçalera no s'ha de creure: %s", got)
	}
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.9")
	if got := clientIP(r); got != "198.51.100.9" {
		t.Fatalf("darrere de Caddy, val l'última adreça: %s", got)
	}
	r.Header.Del("X-Forwarded-For")
	if got := clientIP(r); got != "127.0.0.1" {
		t.Fatalf("sense capçalera: %s", got)
	}
}

// TestLimitGlobalDIntents: moltes adreces diferents tampoc no poden anar
// provant contrasenyes sense fre.
func TestLimitGlobalDIntents(t *testing.T) {
	u := &Users{intents: map[string]*intentsIP{}}
	for i := 0; i < loginMaxGlobal; i++ {
		u.Fallit("10.0.0." + string(rune('a'+i%26)) + string(rune('a'+i/26)))
	}
	if u.Permes("192.0.2.200") {
		t.Fatal("passat el sostre global, ningú no ha de poder provar més en aquesta finestra")
	}
}

func TestCapcaleresSegures(t *testing.T) {
	h := ambCapcaleresSegures(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("capçaleres: %v", rec.Header())
	}
}

// TestFeinaDeCollaInterrompudaDespresDeReiniciar: el pas es desa al disc i,
// si el servidor es reinicia a mitja feina, l'estat diu «interrupted».
func TestFeinaDeCollaInterrompudaDespresDeReiniciar(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	job := &crewJob{path: crewJobPath(s, "rot-1")}
	job.set("working")
	if _, err := os.Stat(filepath.Join(s.sessDir(), "crew-jobs", "rot-1.json")); err != nil {
		t.Fatal(err)
	}
	// «Reinici»: la feina ja no és a la memòria.
	crewJobs.Delete("rot-1")
	rec := httptest.NewRecorder()
	s.handleCrewStatus(rec, httptest.NewRequest("GET", "/api/winds/crew/status?rotllana=rot-1", nil))
	if body := rec.Body.String(); !strings.Contains(body, `"step":"interrupted"`) || !strings.Contains(body, `"running":false`) {
		t.Fatalf("estat: %s", body)
	}
	job.set("done")
	rec = httptest.NewRecorder()
	s.handleCrewStatus(rec, httptest.NewRequest("GET", "/api/winds/crew/status?rotllana=rot-1", nil))
	if !strings.Contains(rec.Body.String(), `"step":"done"`) {
		t.Fatalf("una feina acabada es queda acabada: %s", rec.Body.String())
	}
}
