package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gregal/internal/config"
)

// Isolates discovery overhead on a cold cache; it does not measure generation
// speed or compare the quality of model-generated results.
func BenchmarkWindowDiscoveryScope(b *testing.B) {
	serve := func(delay time.Duration) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"demo","context_length":32768}]}`))
		}))
	}
	active, unrelated := serve(0), serve(25*time.Millisecond)
	defer active.Close()
	defer unrelated.Close()
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"active": {BaseURL: active.URL}, "unrelated": {BaseURL: unrelated.URL},
		},
		Roles: map[string]config.Role{
			"code": {Provider: "active", Model: "demo"},
			"chat": {Provider: "unrelated", Model: "demo"},
		},
	}
	b.Cleanup(ResetWindows)
	for _, scope := range []string{"all-roles", "active-role"} {
		b.Run(scope, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ResetWindows()
				if scope == "all-roles" {
					DetectWindows(context.Background(), cfg)
				} else {
					DetectRoleWindows(context.Background(), cfg, cfg.Roles["code"])
				}
			}
		})
	}
}
