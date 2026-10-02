package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirsLlistaCarpetesIRepos(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "zeta"), 0o755)
	os.MkdirAll(filepath.Join(root, "alfa", ".git"), 0o755)
	os.MkdirAll(filepath.Join(root, ".amagat"), 0o755)
	os.MkdirAll(filepath.Join(root, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(root, "fitxer.txt"), []byte("x"), 0o600)
	s := goalTestServer(t)
	rec := httptest.NewRecorder()
	s.handleDirs(rec, httptest.NewRequest(http.MethodGet, "/api/dirs?path="+root, nil))
	if rec.Code != 200 {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Path   string     `json:"path"`
		Parent string     `json:"parent"`
		Dirs   []dirEntry `json:"dirs"`
		Hidden int        `json:"hidden"`
		Roots  []string   `json:"roots"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Dirs) != 2 || out.Dirs[0].Name != "alfa" || !out.Dirs[0].Git || out.Dirs[1].Name != "zeta" || out.Dirs[1].Git {
		t.Fatalf("dirs: %+v", out.Dirs)
	}
	if out.Hidden != 2 || out.Parent == "" || len(out.Roots) == 0 {
		t.Fatalf("hidden=%d parent=%q roots=%v", out.Hidden, out.Parent, out.Roots)
	}
	// Sense path: la carpeta de la sessió.
	rec = httptest.NewRecorder()
	s.handleDirs(rec, httptest.NewRequest(http.MethodGet, "/api/dirs", nil))
	if rec.Code != 200 {
		t.Fatalf("sense path: HTTP %d", rec.Code)
	}
	// Un fitxer o una ruta inexistent: 400.
	rec = httptest.NewRecorder()
	s.handleDirs(rec, httptest.NewRequest(http.MethodGet, "/api/dirs?path="+filepath.Join(root, "fitxer.txt"), nil))
	if rec.Code != 400 {
		t.Fatalf("fitxer: HTTP %d", rec.Code)
	}
}
