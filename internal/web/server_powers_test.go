package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/procs"
)

// Els poders i les notes del servidor només arriben als vents de
// l'administrador, i només quan s'han activat.
func TestServerPowersOnlyForAdminWinds(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	h := testHubWithUsers(t)
	s := h.def
	if s.windServerBrief() != "" || s.windLoopScope().policy.ServerPowers {
		t.Fatal("powers must be off by default")
	}
	if err := setServerPowers(true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverNotesFile(), []byte("- blog: /srv/www/blog, served by caddy at blog.example.com"), 0o600); err != nil {
		t.Fatal(err)
	}
	brief := s.windServerBrief()
	if !strings.Contains(brief, "SERVER POWERS") || !strings.Contains(brief, "/srv/www/blog") {
		t.Fatalf("admin brief: %q", brief)
	}
	if pol := s.windLoopScope().policy; !pol.ServerPowers || pol.ServerHome == "" {
		t.Fatal("admin wind scope must carry server powers")
	}
	marta := &Server{user: &User{Name: "marta"}}
	if marta.hasServerPowers() || marta.windServerBrief() != "" {
		t.Fatal("a non-admin account never gets server powers")
	}
}

// fakeCloudflared escriu un script que fa de cloudflared: imprimeix un enllaç
// ràpid i espera, i desa els arguments per comprovar-los.
func fakeCloudflared(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "cloudflared")
	body := "#!/bin/sh\necho \"$@\" >> '" + filepath.ToSlash(argsFile) + "'\n" +
		"echo 'INF |  https://quiet-river-1234.trycloudflare.com  |' >&2\nsleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(script), argsFile
}

func TestTunnelQuickAndNamed(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	bin, _ := fakeCloudflared(t)
	var routed []string
	m := &tunnelManager{store: procs.New(), runs: map[string]*tunnelRun{}, recs: map[string]*tunnelRecord{},
		bin: func() (string, error) { return bin, nil },
		route: func(_, tunnel, host string) (string, error) {
			routed = append(routed, tunnel+" "+host)
			return "ok", nil
		}}
	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, r := range m.runs {
			if r.proc != nil {
				r.gen++
				r.proc.Kill()
			}
		}
	})

	link, err := m.start("aleix", "demo", "3000", "", "")
	if err != nil {
		t.Fatalf("quick tunnel: %v", err)
	}
	if link != "https://quiet-river-1234.trycloudflare.com" {
		t.Fatalf("quick link = %q", link)
	}

	if _, err := m.start("aleix", "blog", "8080", "blog.example.com", ""); err == nil {
		t.Fatal("a hostname without a named tunnel must explain how to create one")
	}
	link, err = m.start("aleix", "blog", "8080", "blog.example.com", "casa")
	if err != nil || link != "https://blog.example.com" {
		t.Fatalf("named tunnel: %q %v", link, err)
	}
	if len(routed) != 1 || routed[0] != "casa blog.example.com" {
		t.Fatalf("DNS route = %v", routed)
	}
	cfg, err := os.ReadFile(tunnelConfigFile())
	if err != nil || !strings.Contains(string(cfg), "hostname: blog.example.com") || !strings.Contains(string(cfg), "service: http://127.0.0.1:8080") || !strings.Contains(string(cfg), "http_status:404") {
		t.Fatalf("ingress config:\n%s (%v)", cfg, err)
	}
	if l := m.list("aleix"); len(l) != 2 {
		t.Fatalf("list = %v", l)
	}
	if l := m.list("marta"); len(l) != 0 {
		t.Fatal("tunnels are per account")
	}
	if _, err := m.start("aleix", "x", "http://10.0.0.5:80", "", ""); err == nil {
		t.Fatal("a tunnel only exposes services of this machine")
	}

	// Es desen i tornen després de reiniciar.
	again := &tunnelManager{store: procs.New(), runs: map[string]*tunnelRun{}, recs: map[string]*tunnelRecord{},
		bin: func() (string, error) { return bin, nil }, route: m.route}
	again.boot()
	if len(again.recs) != 2 {
		t.Fatalf("restored %d tunnels, want 2", len(again.recs))
	}
	if _, err := m.stop("aleix", "blog.example.com"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if l := m.list("aleix"); len(l) != 1 {
		t.Fatalf("after stop: %v", l)
	}
}
