package tools

// Prova viva de punta a punta: un Chrome de debò amb l'extensió de Gregal
// carregada (una còpia amb el permís de depuració ja concedit i apuntant al
// port de la prova), que es connecta al backend i deixa que l'agent faci
// servir el navegador. Salta sense GREGAL_LIVE_EXT=1.
//
// Ús: GREGAL_LIVE_EXT=1 go test ./internal/tools/ -run TestLiveChromeExtension -v
//
// Chrome de marca (137+) ja no carrega extensions per línia d'ordres; Edge sí:
// GREGAL_BROWSER="C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe".

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(filepath.Join(dst, rel))
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func patchFile(t *testing.T, path, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), from) {
		t.Fatalf("%s: no trobo %q", path, from)
	}
	os.WriteFile(path, []byte(strings.Replace(string(raw), from, to, 1)), 0o644)
}

func TestLiveChromeExtension(t *testing.T) {
	if os.Getenv("GREGAL_LIVE_EXT") == "" {
		t.Skip("sense GREGAL_LIVE_EXT=1")
	}
	exe := BrowserExecutable()
	if exe == "" {
		t.Skip("cap Chrome/Edge")
	}
	// Backend: el websocket de l'extensió en un port propi.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.Handle("/api/browser/extension", websocket.Server{
		Handshake: func(c *websocket.Config, r *http.Request) error {
			if !strings.HasPrefix(r.Header.Get("Origin"), "chrome-extension://") {
				return fmt.Errorf("origin %q", r.Header.Get("Origin"))
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) { ServeBrowserExtension(ws, nil) },
	})
	back := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	back.Start()
	defer back.Close()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fixtureHTML)
	}))
	defer site.Close()

	// Còpia de l'extensió: permís concedit i port de la prova.
	_, here, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(here), "..", "..", "extension", "chrome")
	extRoot, _ := os.MkdirTemp("", "gregal-ext-src-")
	defer os.RemoveAll(extRoot)
	ext := filepath.Join(extRoot, "ext")
	copyDir(t, src, ext)
	patchFile(t, filepath.Join(ext, "manifest.json"), `"optional_permissions": [`+"\n"+`    "debugger"`+"\n"+`  ]`, `"optional_permissions": []`)
	patchFile(t, filepath.Join(ext, "manifest.json"), `"alarms"`, `"alarms", "debugger"`)
	patchFile(t, filepath.Join(ext, "background.js"), "return !!c.control &&", "return true ||")
	patchFile(t, filepath.Join(ext, "logic.js"), "conf.desktopPort || '8097'", fmt.Sprintf("'%d'", port))

	// El perfil no va a t.TempDir: el navegador tarda a deixar anar els
	// fitxers en tancar-se i la neteja automàtica fallaria la prova.
	profile, _ := os.MkdirTemp("", "gregal-ext-live-")
	defer func() { time.Sleep(time.Second); os.RemoveAll(profile) }()
	cmd := exec.Command(exe, "--user-data-dir="+profile, "--load-extension="+ext,
		"--disable-features=DisableLoadExtensionCommandLineSwitch", "--no-first-run", "--no-default-browser-check",
		"--headless=new", "--window-size=1280,900", "about:blank")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	defer CloseBrowser()

	deadline := time.Now().Add(30 * time.Second)
	for BrowserExtensionStatus()["connected"] != true {
		if time.Now().After(deadline) {
			t.Fatal("l'extensió no s'ha connectat en 30 s (Chrome no ha carregat l'extensió?)")
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Logf("extensió connectada: %v", BrowserExtensionStatus())

	const scope = "live-ext"
	defer ForgetBrowserSites(scope)
	do := func(args map[string]any) string {
		t.Helper()
		act, _ := args["action"].(string)
		t0 := time.Now()
		out, _, err := BrowserActionFor(context.Background(), scope, act, args)
		if err != nil {
			t.Fatalf("%s: %v", act, err)
		}
		t.Logf("── %s (%v)\n%s", act, time.Since(t0).Round(time.Millisecond), out)
		return out
	}
	page := do(map[string]any{"action": "open", "url": site.URL + "/", "chrome": true})
	if !strings.Contains(page, `textbox "Nom"`) {
		t.Fatalf("lectura al Chrome de l'usuari:\n%s", page)
	}
	nom, envia := refOf(t, page, `textbox "Nom"`), refOf(t, page, `button "Envia la comanda"`)
	out := do(map[string]any{"action": "batch", "steps": []any{
		map[string]any{"action": "type", "target": nom, "text": "Pau"},
		map[string]any{"action": "click", "target": envia},
	}})
	if !strings.Contains(out, "Enviat: Pau") {
		t.Fatalf("el formulari s'havia d'enviar al Chrome de l'usuari:\n%s", out)
	}
	if out = do(map[string]any{"action": "tabs"}); !strings.Contains(out, "Botiga de prova") {
		t.Fatalf("tabs del grup Gregal: %s", out)
	}
}
