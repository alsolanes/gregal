package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPlot = `{"title":"Sales","type":"line","labels":["Jan","Feb"],"series":[{"label":"EUR","values":[10,20]}]}`

func TestPlotsPersistAndRejectStaleUpdates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	s := &Server{cwd: t.TempDir(), id: "origin"}
	request := func(method, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest(method, "/api/plots", strings.NewReader(body)))
		return w
	}
	w := request("POST", `{"revision":0,"spec":`+testPlot+`,"dashboard":true}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var lib plotLibrary
	json.Unmarshal(w.Body.Bytes(), &lib)
	if lib.Revision != 1 || len(lib.Plots) != 1 || lib.Plots[0].Session != "origin" || !lib.Plots[0].Dashboard {
		t.Fatalf("bad library: %+v", lib)
	}
	id := lib.Plots[0].ID
	if w = request("PATCH", `{"revision":0,"id":"`+id+`","width":2}`); w.Code != 409 {
		t.Fatal("stale write accepted", w.Code)
	}
	if w = request("PATCH", `{"revision":1,"id":"`+id+`","width":2,"dashboard":true,"note":"Keep this"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// A new server for the same project reads the on-disk library.
	other := &Server{cwd: s.cwd, id: "another"}
	w = httptest.NewRecorder()
	other.handlePlots(w, httptest.NewRequest("GET", "/api/plots", nil))
	json.Unmarshal(w.Body.Bytes(), &lib)
	if lib.Revision != 2 || lib.Plots[0].Width != 2 || lib.Plots[0].Note != "Keep this" {
		t.Fatalf("did not persist: %+v", lib)
	}
	if w = request("DELETE", `{"revision":2,"id":"`+id+`"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &lib)
	if len(lib.Plots) != 0 || lib.Revision != 3 {
		t.Fatal(lib)
	}
}

func TestNamedDashboardsMigrateAndPreservePlotsOnDelete(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := &Server{cwd: t.TempDir(), id: "a"}
	path := s.plotStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"revision":4,"plots":[{"id":"old","spec":` + testPlot + `,"dashboard":true,"width":1}]}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	call := func(method string, body any) plotLibrary {
		raw, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest(method, "/api/plots", strings.NewReader(string(raw))))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var lib plotLibrary
		if err := json.Unmarshal(w.Body.Bytes(), &lib); err != nil {
			t.Fatal(err)
		}
		return lib
	}
	lib := call("GET", nil)
	if len(lib.Boards) != 1 || lib.Plots[0].Board != "default" || !lib.Plots[0].Dashboard {
		t.Fatal("legacy dashboard lost", lib)
	}
	lib = call("POST", map[string]any{"revision": 4, "operation": "create_board", "name": "Revenue"})
	board := lib.Boards[1].ID
	lib = call("PATCH", map[string]any{"revision": 5, "id": "old", "width": 2, "dashboard": true, "board": board, "note": "Keep me"})
	if lib.Plots[0].Board != board {
		t.Fatal("plot was not moved")
	}
	lib = call("POST", map[string]any{"revision": 6, "operation": "rename_board", "board": board, "name": "Finance"})
	if lib.Boards[1].Name != "Finance" {
		t.Fatal("rename failed")
	}
	lib = call("POST", map[string]any{"revision": 7, "operation": "delete_board", "board": board})
	if len(lib.Boards) != 1 || len(lib.Plots) != 1 || lib.Plots[0].Dashboard || lib.Plots[0].Note != "Keep me" || lib.Plots[0].Spec.Title != "Sales" {
		t.Fatal("deleting dashboard lost library data", lib)
	}
	for _, body := range []string{`{"revision":8,"operation":"delete_board","board":"default"}`, `{"revision":8,"operation":"create_board","name":" "}`, `{"revision":8,"operation":"unknown"}`} {
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest("POST", "/api/plots", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal("bad board operation accepted", w.Code)
		}
	}
}

func TestPlotsIsolationValidationAndReorder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	s := &Server{cwd: root, id: "a", user: &User{Name: "alice"}}
	other := &Server{cwd: root, user: &User{Name: "bob"}}
	if s.plotStorePath() == other.plotStorePath() {
		t.Fatal("users share plots")
	}
	other.user = s.user
	other.cwd = filepath.Join(root, "another")
	if s.plotStorePath() == other.plotStorePath() {
		t.Fatal("projects share plots")
	}
	call := func(body string, method string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest(method, "/api/plots", strings.NewReader(body)))
		return w
	}
	for _, bad := range []string{
		`{"revision":0,"spec":{"title":"Bad","type":"html","labels":["x"],"series":[{"label":"a","values":[1]}]}}`,
		`{"revision":0,"spec":{"title":"Bad","type":"bar","labels":["x"],"series":[{"label":"a","values":[]}]}}`,
		`{"revision":0,"spec":` + testPlot + `,"code":"alert(1)"}`,
		`{"revision":0,"spec":` + testPlot + `} {}`,
		`{"revision":0,"spec":{"title":"Bad","type":"line","labels":["x"],"series":[{"label":"a","values":[null]}]}}`,
	} {
		if w := call(bad, "POST"); w.Code != 400 {
			t.Fatalf("accepted bad spec: %s: %d", bad, w.Code)
		}
	}
	for i := 0; i < 2; i++ {
		raw, _ := json.Marshal(map[string]any{"revision": i, "spec": json.RawMessage(testPlot)})
		if w := call(string(raw), "POST"); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := call("", "GET")
	var lib plotLibrary
	json.Unmarshal(w.Body.Bytes(), &lib)
	first := lib.Plots[0].ID
	w = call(`{"revision":2,"id":"`+first+`","width":1,"direction":1}`, "PATCH")
	json.Unmarshal(w.Body.Bytes(), &lib)
	if w.Code != 200 || lib.Plots[1].ID != first {
		t.Fatal("reorder failed", w.Code, w.Body.String())
	}
	if w := call(`{"revision":3,"id":"missing"}`, "DELETE"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := call("", "PUT"); w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestDashboardReorderSkipsLibraryOnlyPlots(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := &Server{cwd: t.TempDir(), id: "a"}
	call := func(method string, body any) plotLibrary {
		raw, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest(method, "/api/plots", strings.NewReader(string(raw))))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var lib plotLibrary
		if err := json.Unmarshal(w.Body.Bytes(), &lib); err != nil {
			t.Fatal(err)
		}
		return lib
	}
	var lib plotLibrary
	for i := 0; i < 3; i++ {
		lib = call("POST", map[string]any{"revision": i, "spec": json.RawMessage(testPlot), "dashboard": i != 1})
	}
	first, middle, last := lib.Plots[0].ID, lib.Plots[1].ID, lib.Plots[2].ID
	lib = call("PATCH", map[string]any{"revision": 3, "id": first, "width": 1, "dashboard": true, "direction": 1, "dashboard_only": true})
	if lib.Plots[0].ID != last || lib.Plots[1].ID != middle || lib.Plots[2].ID != first {
		t.Fatal("visible dashboard order did not change", lib)
	}
}
