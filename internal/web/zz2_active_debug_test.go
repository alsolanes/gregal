package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

func TestZZ2ActivaSeq(t *testing.T) {
	var n int64
	started := make(chan struct{})
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := atomic.AddInt64(&n, 1)
		t.Logf("ZZ2 crida %d: %s %s", i, r.Method, r.URL.Path)
		if i == 1 {
			close(started)
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Fet."}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	done := make(chan struct{})
	go func() {
		s.handleAgent(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"revisa ara"}`)))
		close(done)
	}()
	<-started
	rec := httptest.NewRecorder()
	s.handleActive(rec, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	close(release)
	<-done
	var out struct {
		Active *activeAgent `json:"active"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	t.Logf("ZZ2 total crides: %d", atomic.LoadInt64(&n))
}
