package agent

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Un proveïdor remot sondejat en un procés no es torna a sondejar en el
// següent (el disc ho recorda), i la finestra que va dir torna a ser
// coneguda sense cap petició.
func TestWindowDiskRecordaElSondeigEntreProcessos(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(`{"data":[{"id":"m1","context_length":200000},{"id":"m2"}]}`))
	}))
	defer srv.Close()
	// httptest escolta a 127.0.0.1, que comptaria com a local: el sondeig
	// remot es prova amb un altre nom de la mateixa adreça.
	base := "http://remote.test:" + srv.URL[len("http://127.0.0.1:"):] + "/v1"
	old := http.DefaultTransport
	http.DefaultTransport = &http.Transport{Proxy: nil, DialContext: dialA(srv.Listener.Addr().String())}
	defer func() { http.DefaultTransport = old }()
	if esServidorLocal(base) {
		t.Fatalf("%s no hauria de ser local", base)
	}

	WindowsPathOverride = filepath.Join(t.TempDir(), "windows.json")
	defer func() { WindowsPathOverride = "" }()
	ResetWindows()
	defer ResetWindows()

	detectWindowBases(t.Context(), map[string]*baseProbe{base: {models: []string{"m1"}}})
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("primer sondeig: %d peticions", hits)
	}
	// «Procés nou»: res en memòria, només el fitxer.
	ResetWindows()
	detectWindowBases(t.Context(), map[string]*baseProbe{base: {models: []string{"m2"}}})
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("el segon procés ha tornat a sondejar (%d peticions)", hits)
	}
	if n, ok := KnownWindow(base, "m1"); !ok || n != 200000 {
		t.Fatalf("finestra recordada = %d, %v", n, ok)
	}
}

// dialA connecta qualsevol host a addr (per fer passar un httptest per remot).
func dialA(addr string) func(ctx context.Context, network, _ string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
}

// En obrir una conversa (abans de cap torn ni sondeig) la finestra desada ja
// es veu, encara que el sondeig sigui de fa dies: abans l'escriptori ensenyava
// el defecte de 32K amb halogen (131072) fins que el primer torn sondejava.
func TestWindowDiskEsVeuSenseSondejarEncaraQueSiguiVell(t *testing.T) {
	WindowsPathOverride = filepath.Join(t.TempDir(), "windows.json")
	defer func() { WindowsPathOverride = "" }()
	base := "https://llm.example.test/v1"
	vell := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	raw := `{"probed":{"` + base + `":"` + vell + `"},"windows":{"` + base + `|halogen-x":131072}}`
	if err := os.WriteFile(WindowsPathOverride, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	ResetWindows()
	defer ResetWindows()
	if n, ok := KnownWindow(base, "halogen-x"); !ok || n != 131072 {
		t.Fatalf("finestra desada = %d, %v", n, ok)
	}
	// Vell: s'ha de tornar a preguntar (per si ha canviat), però mentrestant ja es veu.
	if sondejatRecent(base) {
		t.Fatal("un sondeig de fa tres dies no és recent")
	}
}
