package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/tools"
)

func fileTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	setHomeTest(t, t.TempDir())
	s := hubTestServer(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hola\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "node_modules", "x", "junk.js"), []byte("x\n"), 0o644)
	if err := s.setWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// L'arbre llista el workspace i salta node_modules i companyia.
func TestHandleTree(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleTree(w, httptest.NewRequest("GET", "/api/tree?depth=2", nil))
	var out struct {
		Entries []treeNode `json:"entries"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	names := map[string]treeNode{}
	for _, e := range out.Entries {
		names[e.Name] = e
	}
	if _, ok := names["node_modules"]; ok {
		t.Fatal("node_modules no s'ha de llistar")
	}
	if _, ok := names["a.go"]; !ok {
		t.Fatalf("falta a.go: %+v", out.Entries)
	}
	sub, ok := names["sub"]
	if !ok || !sub.Dir || len(sub.Children) != 1 || sub.Children[0].Path != "sub/b.txt" {
		t.Fatalf("subdirectori mal format: %+v", sub)
	}
	// Els directoris van primer.
	if !out.Entries[0].Dir {
		t.Fatalf("els directoris van primer: %+v", out.Entries)
	}
}

func TestHandleTreeNestedPath(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleTree(w, httptest.NewRequest("GET", "/api/tree?path=sub&depth=1", nil))
	var out struct {
		Entries []treeNode `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Entries) != 1 || out.Entries[0].Path != "sub/b.txt" {
		t.Fatalf("ruta de fitxer imbricat: %+v", out.Entries)
	}
}

// Cap ruta pot sortir del workspace.
func TestSafeJoin(t *testing.T) {
	s, dir := fileTestServer(t)
	if _, err := s.safeJoin("../secret"); err == nil {
		t.Fatal("..  hauria de fallar")
	}
	// Arrels sense unitat: filepath.IsAbs a Windows hi diu que no i abans
	// s'unien al workspace tan tranquil·lament.
	for _, p := range []string{`\Windows\System32\drivers\etc\hosts`, "C:fitxer", `\\servidor\recurs`} {
		if _, err := s.safeJoin(p); err == nil {
			t.Fatalf("%q havia de quedar fora del workspace", p)
		}
	}
	if _, err := s.safeJoin("/etc/passwd"); err == nil {
		t.Fatal("una ruta absoluta hauria de fallar")
	}
	got, err := s.safeJoin("sub/b.txt")
	if err != nil || got != filepath.Join(dir, "sub", "b.txt") {
		t.Fatalf("%q %v", got, err)
	}
}

// La contenció lèxica no és suficient: un symlink dins del workspace no pot
// convertir /api/file en un lector de fitxers arbitraris del disc.
func TestSafeJoinRejectsSymlinkOutsideWorkspace(t *testing.T) {
	s, dir := fileTestServer(t)
	external := t.TempDir()
	secret := filepath.Join(external, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET-CONTENT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "external")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("el sistema no permet crear symlinks: %v", err)
	}
	if _, err := s.safeJoin(filepath.Join("external", "secret.txt")); err == nil {
		t.Fatal("safeJoin ha acceptat un symlink que surt del workspace")
	}
	w := httptest.NewRecorder()
	s.handleFile(w, httptest.NewRequest("GET", "/api/file?path=external/secret.txt", nil))
	if w.Code == 200 || strings.Contains(w.Body.String(), "TOP-SECRET-CONTENT") {
		t.Fatalf("/api/file ha travessat el symlink: status=%d body=%s", w.Code, w.Body.String())
	}
}

// /api/file retorna el contingut, i marca binaris i fitxers massa grossos.
func TestHandleFile(t *testing.T) {
	s, dir := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleFile(w, httptest.NewRequest("GET", "/api/file?path=a.go", nil))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["content"] != "package a\n" {
		t.Fatalf("%v", out)
	}
	os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0x00, 0x01, 0x02}, 0o644)
	w2 := httptest.NewRecorder()
	s.handleFile(w2, httptest.NewRequest("GET", "/api/file?path=bin.dat", nil))
	json.Unmarshal(w2.Body.Bytes(), &out)
	if out["binary"] != true {
		t.Fatalf("binari: %v", out)
	}
	w3 := httptest.NewRecorder()
	s.handleFile(w3, httptest.NewRequest("GET", "/api/file?path=no.txt", nil))
	if w3.Code != 404 {
		t.Fatalf("inexistent: %d", w3.Code)
	}
	w4 := httptest.NewRecorder()
	s.handleFile(w4, httptest.NewRequest("GET", "/api/file?path=../fora.txt", nil))
	if w4.Code != 400 {
		t.Fatalf("fora del projecte: %d", w4.Code)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

// /api/diff dona els canvis amb hunks i /api/diff/discard els desfà.
func TestHandleDiffIDescarta(t *testing.T) {
	s, dir := fileTestServer(t)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o644)
	gitInit(t, dir)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() { println(1) }\n"), 0o644)

	w := httptest.NewRecorder()
	s.handleDiff(w, httptest.NewRequest("GET", "/api/diff", nil))
	var out struct {
		Files   []tools.DiffFile `json:"files"`
		Added   int              `json:"added"`
		Removed int              `json:"removed"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Files) != 1 || out.Added != 1 || out.Removed != 1 {
		t.Fatalf("%+v", out)
	}
	if len(out.Files[0].Hunks) == 0 || !strings.Contains(out.Files[0].Hunks[0].Patch, "println") {
		t.Fatalf("hunk: %+v", out.Files[0])
	}

	w2 := httptest.NewRecorder()
	s.handleDiffDiscard(w2, httptest.NewRequest("POST", "/api/diff/discard", strings.NewReader(`{"path":"a.go","hunk":0}`)))
	if w2.Code != 200 {
		t.Fatalf("descarta: %d %s", w2.Code, w2.Body.String())
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "a.go"))
	if strings.Contains(string(raw), "println") {
		t.Fatalf("el canvi no s'ha descartat: %s", raw)
	}
}

// Descartar un fitxer nou l'esborra.
func TestDescartaFitxerNou(t *testing.T) {
	s, dir := fileTestServer(t)
	gitInit(t, dir)
	os.WriteFile(filepath.Join(dir, "nou.txt"), []byte("hola\n"), 0o644)
	w := httptest.NewRecorder()
	s.handleDiffDiscard(w, httptest.NewRequest("POST", "/api/diff/discard", strings.NewReader(`{"path":"nou.txt"}`)))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "nou.txt")); !os.IsNotExist(err) {
		t.Fatal("el fitxer nou s'havia d'esborrar")
	}
}

// Descartar un fitxer sense canvis és 404, no un 500.
func TestDescartaSenseCanvis(t *testing.T) {
	s, dir := fileTestServer(t)
	gitInit(t, dir)
	w := httptest.NewRecorder()
	s.handleDiffDiscard(w, httptest.NewRequest("POST", "/api/diff/discard", strings.NewReader(`{"path":"a.go"}`)))
	if w.Code != 404 {
		t.Fatalf("%d", w.Code)
	}
}

// resolveFilePath ha de permetre carpetes externes i expansió de ~.
func TestResolveFilePath_ExternalAndHome(t *testing.T) {
	s, dir := fileTestServer(t)
	extDir := t.TempDir()
	extFile := filepath.Join(extDir, "extern.txt")
	os.WriteFile(extFile, []byte("contingut extern\n"), 0o644)

	// Ruta absoluta directa
	resolved, err := s.resolveFilePath(extFile)
	if err != nil || resolved != extFile {
		t.Fatalf("esperava %q, obtingut %q (err: %v)", extFile, resolved, err)
	}

	// Arbre d'una carpeta externa
	w := httptest.NewRecorder()
	s.handleTree(w, httptest.NewRequest("GET", "/api/tree?depth=2&path="+extDir, nil))
	if w.Code != 200 {
		t.Fatalf("tree extDir: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Entries []treeNode `json:"entries"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Entries) != 1 || out.Entries[0].Name != "extern.txt" {
		t.Fatalf("arbre extern incorrecte: %+v", out.Entries)
	}

	// Lectura de fitxer extern
	wFile := httptest.NewRecorder()
	s.handleFile(wFile, httptest.NewRequest("GET", "/api/file?path="+extFile, nil))
	if wFile.Code != 200 {
		t.Fatalf("file ext: %d %s", wFile.Code, wFile.Body.String())
	}
	var fileRes struct {
		Content string `json:"content"`
	}
	json.Unmarshal(wFile.Body.Bytes(), &fileRes)
	if fileRes.Content != "contingut extern\n" {
		t.Fatalf("contingut no coincideix: %q", fileRes.Content)
	}

	// Dins del workspace relatiu continua funcionant
	rel, err := s.resolveFilePath("sub/b.txt")
	if err != nil || rel != filepath.Join(dir, "sub", "b.txt") {
		t.Fatalf("relatiu workspace ha fallat: %q %v", rel, err)
	}
}
