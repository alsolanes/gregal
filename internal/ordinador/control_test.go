package ordinador

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// B3 — lease de control i sessions del visor. No hi ha VM real: el
// hipervisor i l'agent són dobles (newFakeVM, httptest).

func TestLeasePersistsAndExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	clock := time.Now()
	l := NewLeases(path)
	l.SetClock(func() time.Time { return clock })
	x := l.Take("k-carla", "handoff", "wa-1")
	if x.Holder != "user" || x.Handoff != "wa-1" || !l.HeldByUser("k-carla") || l.HeldByUser("k-aleix") {
		t.Fatalf("lease: %+v", x)
	}
	// Reinici de l'amfitrió: la lease hi és, amb la mateixa caducitat.
	l2 := NewLeases(path)
	l2.SetClock(func() time.Time { return clock })
	if got := l2.Get("k-carla"); got == nil || got.Handoff != "wa-1" || !got.Expires.Equal(x.Expires) {
		t.Fatalf("després del reinici: %+v", got)
	}
	// Sense renovar (l'app tancada), caduca sola: ningú no queda amb el
	// control penjat.
	clock = clock.Add(UserLeaseTTL + time.Second)
	if l2.HeldByUser("k-carla") {
		t.Fatal("la lease havia de caducar")
	}
	if _, ok := l2.Renew("k-carla"); ok {
		t.Fatal("renovar no crea una lease caducada")
	}
	l2.Take("k-carla", "viewer", "")
	clock = clock.Add(UserLeaseTTL - time.Second)
	if _, ok := l2.Renew("k-carla"); !ok {
		t.Fatal("renovar dins del termini")
	}
	clock = clock.Add(UserLeaseTTL - time.Second)
	if !l2.HeldByUser("k-carla") {
		t.Fatal("la renovació allarga la lease")
	}
	if !l2.Release("k-carla") || l2.HeldByUser("k-carla") || l2.Release("k-carla") {
		t.Fatal("retornar el control")
	}
	if NewLeases(path).HeldByUser("k-carla") {
		t.Fatal("el retorn també queda desat")
	}
}

func TestViewerTicketsAreOneShotAndShortLived(t *testing.T) {
	clock := time.Now()
	v := NewViewers()
	v.SetClock(func() time.Time { return clock })
	tk, exp := v.Issue("k-carla", "carla", "pc-carla")
	if len(tk) < 64 || !exp.Equal(clock.Add(ViewerTicketTTL)) {
		t.Fatalf("testimoni: %q %v", tk, exp)
	}
	s, ok := v.Redeem(tk)
	if !ok || s.Owner != "carla" || s.Key != "k-carla" || s.Computer != "pc-carla" || s.ID == tk || s.ID == "" {
		t.Fatalf("sessió: %+v", s)
	}
	if _, ok := v.Redeem(tk); ok {
		t.Fatal("el testimoni és d'un sol ús")
	}
	if _, ok := v.Redeem(""); ok {
		t.Fatal("testimoni buit")
	}
	tk2, _ := v.Issue("k-carla", "carla", "pc-carla")
	clock = clock.Add(ViewerTicketTTL + time.Second)
	if _, ok := v.Redeem(tk2); ok {
		t.Fatal("el testimoni caduca si no es fa servir")
	}
	// La sessió caduca sense batec i té un topall absolut.
	clock = clock.Add(ViewerSessionTTL)
	if _, ok := v.Session(s.ID, false); ok {
		t.Fatalf("sense batec en %v la sessió havia de caducar", ViewerSessionTTL)
	}
	tk3, _ := v.Issue("k-carla", "carla", "pc-carla")
	s3, _ := v.Redeem(tk3)
	for i := 0; i < int(ViewerSessionMax/ViewerSessionTTL)+2; i++ {
		clock = clock.Add(ViewerSessionTTL - time.Second)
		if _, ok := v.Session(s3.ID, true); !ok {
			if clock.Sub(s3.Created) < ViewerSessionMax {
				t.Fatalf("el batec manté la sessió (%d)", i)
			}
			return
		}
	}
	t.Fatal("la sessió del visor ha de tenir un topall absolut")
}

func TestUserControlBlocksAgentAndKeepsAwake(t *testing.T) {
	f, m, c := newFakeVM(t)
	l := NewLeases(filepath.Join(t.TempDir(), "control.json"))
	m.SetLeases(l)
	clock := time.Now()
	m.now = func() time.Time { return clock }
	l.SetClock(func() time.Time { return clock })
	m.Wake(context.Background(), c, "viewer")

	l.Take(c.LeaseKey(), "viewer", "")
	r := m.Status(context.Background(), c)
	if r.ControlOwner != "user" || r.LeaseExpiry == "" {
		t.Fatalf("la salut ha de dir que la persona té el control: %+v", r)
	}
	// L'agent no toca el navegador (ni captures) mentre la persona el té;
	// les altres eines de l'ordinador no depenen de la pantalla.
	if _, err := m.Exec(context.Background(), "carla", ExecRequest{Tool: "browser", Args: `{"action":"screenshot"}`}); !errors.Is(err, ErrUserHasControl) {
		t.Fatalf("browser amb la persona al control: %v", err)
	}
	if out, err := m.Exec(context.Background(), "carla", ExecRequest{Tool: "read"}); err != nil || out.Output != "carla:read" {
		t.Fatalf("read: %v %+v", err, out)
	}
	// Amb el control a la persona no es desa, encara que passin els minuts.
	for i := 0; i < 11; i++ { // el visor renova la lease mentre és a la vista
		clock = clock.Add(time.Minute)
		l.Renew(c.LeaseKey())
	}
	if got := m.ParkIdle(context.Background()); len(got) != 0 {
		t.Fatalf("amb la persona al control no es desa: %v", got)
	}
	// La persona tanca l'app: la lease caduca i el compte enrere torna a
	// córrer des de l'últim ús (no queda encès per un visor oblidat).
	clock = clock.Add(UserLeaseTTL + time.Second)
	if r := m.Status(context.Background(), c); r.ControlOwner != "none" {
		t.Fatalf("lease caducada: %+v", r)
	}
	clock = clock.Add(10 * time.Minute)
	if got := m.ParkIdle(context.Background()); len(got) != 1 || f.power != "saved" {
		t.Fatalf("sense ningú, es desa: %v %s", got, f.power)
	}
}

func TestAgentViewerProxyNeedsTokenAndDropsCookies(t *testing.T) {
	var gotCookie, gotAuth, gotPath string
	novnc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie, gotAuth, gotPath = r.Header.Get("Cookie"), r.Header.Get("Authorization"), r.URL.Path
		http.SetCookie(w, &http.Cookie{Name: "vm", Value: "x"})
		io.WriteString(w, "NOVNC")
	}))
	t.Cleanup(novnc.Close)
	h, err := NewAgent(AgentConfig{ID: "pc-carla", Owner: "carla", FilesDir: t.TempDir(), Token: testToken, ViewerAddr: strings.TrimPrefix(novnc.URL, "http://")})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/v1/viewer/vnc.html", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || gotPath != "" {
		t.Fatalf("sense testimoni: %d %q", w.Code, gotPath)
	}
	r = httptest.NewRequest("GET", "/v1/viewer/vnc.html", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	r.Header.Set("Cookie", "gregal_visor=secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "NOVNC" || gotPath != "/vnc.html" {
		t.Fatalf("visor: %d %q %q", w.Code, w.Body.String(), gotPath)
	}
	if gotCookie != "" || gotAuth != "" || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("ni galetes ni testimoni no passen al noVNC: cookie=%q auth=%q set=%q", gotCookie, gotAuth, w.Header().Get("Set-Cookie"))
	}
	if _, err := NewAgent(AgentConfig{ID: "pc-carla", Owner: "carla", FilesDir: t.TempDir(), Token: testToken, ViewerAddr: "8.8.8.8:6080"}); err == nil {
		t.Fatal("el visor no pot apuntar fora de loopback")
	}
}
