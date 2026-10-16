package agent

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
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
