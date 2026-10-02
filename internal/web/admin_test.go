package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }

// TestAdminOnlyJobsIGithub: les rutes del servidor (jobs, GitHub) són de
// l'amo. Amb dos usuaris, qui no és admin no hi ha d'arribar; en mode local
// (sense autenticació) tot passa com sempre.
func TestAdminOnlyJobsIGithub(t *testing.T) {
	s := goalTestServer(t)
	passa := func(u *User) int {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		if u != nil {
			req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser{}, u))
		}
		rec := httptest.NewRecorder()
		adminOnly(func(_ *Server, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})(s, rec, req)
		return rec.Code
	}

	if c := passa(nil); c != http.StatusOK {
		t.Fatalf("sense autenticació (local) hauria de passar: %d", c)
	}
	if c := passa(&User{Name: "usera", Admin: true}); c != http.StatusOK {
		t.Fatalf("l'admin hauria de passar: %d", c)
	}
	if c := passa(&User{Name: "userb"}); c != http.StatusForbidden {
		t.Fatalf("qui no és admin hauria de rebre 403: %d", c)
	}
}

// TestHomeDeLUsuari: la carpeta de treball mana sobre la primera arrel i ha
// de quedar dins del que l'usuari pot tocar.
func TestHomeDeLUsuari(t *testing.T) {
	base := t.TempDir()
	userb := base + "/userb"
	obres := base + "/userb/obres"
	for _, d := range []string{userb, obres} {
		if err := mkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}
	u := &User{Name: "userb", Roots: []string{userb}, Home: obres}
	if u.Home != obres {
		t.Fatalf("home: %s", u.Home)
	}
	if err := u.Allow(u.Home); err != nil {
		t.Fatalf("la carpeta de treball ha de ser accessible: %v", err)
	}
	// Un home fora de les arrels no hauria de passar mai el guard.
	dolent := &User{Name: "userb", Roots: []string{userb}, Home: base + "/usera"}
	if err := dolent.Allow(dolent.Home); err == nil {
		t.Fatal("un home fora de les arrels ha de quedar bloquejat")
	}
}
